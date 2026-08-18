// Package vault 实现"锁屏密码保险库", 与 LdTools 的 vault 对齐, 但关键差异:
//
//   - LdTools: 锁屏是 UI 级(数据库不加密);
//   - LdSSHManager: 锁屏密码经 scrypt 派生 32 字节 key 作为 SQLCipher 开库 key
//     (PRAGMA key / rekey), 数据库整库加密与锁屏密码绑定。
//
// 统一状态机(2026-08-14 重构, 保证加密逻辑始终自洽):
//   - 密码只存加盐哈希(scrypt), 绝不落盘明文;
//   - 校验物 vault.lock 存放于 data 目录(与数据库同级), 任何状态都永驻不删;
//   - 永远存在一个锁屏密码: 用户从未设置(或已取消)时 = 默认密码 123456,
//     数据库 key 恒为 scrypt(当前密码, salt), 无"无密码/固定 key"游离状态;
//   - enabled 只控制"启动是否弹锁屏": false = 启动用默认密码 key 直接开库直进,
//     true = 启动锁定, 解锁输入用户密码;
//   - 强不变量: enabled=true ⟺ 密码≠默认密码 123456(设置/修改密码禁止用默认密码,
//     老数据 enabled=true 但密码是默认的会在 Init 时修正为未设置并直进);
//   - 123456 是"隐蔽密码"(用户不知道有密码): 只要当前密码是 123456,
//     任何需要密码的界面(锁屏/修改/取消)都自动通过, 永远不让用户输入;
//   - 设置/修改/取消锁屏密码时, 通过注入的 rekey 回调把数据库 key 切换。
package vault

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unicode"

	"golang.org/x/crypto/scrypt"
)

const (
	MinLen = 6
	MaxLen = 64
	// DefaultPassword 默认锁屏密码: 用户从未设置(或已取消)锁屏密码时,
	// 数据库始终用该密码派生的 key 加密, 保证"永远存在一把可解锁的钥匙"。
	DefaultPassword = "123456"
)

var (
	ErrNoPassword     = errors.New("未启用锁屏密码")
	ErrWrongPassword  = errors.New("锁屏密码错误")
	ErrAlreadyEnabled = errors.New("锁屏密码已启用")
	ErrWrongLength    = errors.New("密码长度需为 6-64 位")
	// ErrDefaultPassword 禁止把锁屏密码设为默认密码 123456:
	// 123456 是"未设置"状态的隐蔽密码(用户不知道有密码), 设为它会导致
	// 锁屏要求输入用户不知道的密码; 且默认密码公开, 设了也毫无保护意义。
	ErrDefaultPassword = errors.New("不能使用默认密码 123456, 请换一个")
	// ErrWeakPassword 锁屏密码必须同时包含字母和数字(用户 08-14 要求)。
	ErrWeakPassword = errors.New("密码必须同时包含字母和数字")
)

// lockFile 落盘结构 (vault.lock)
// Hash 即 scrypt(password, salt) 的 hex —— 同时用作 SQLCipher 开库 key,
// 因此"校验密码"与"得到数据库 key"是同一个派生结果, 无第二个秘密。
type lockFile struct {
	Enabled bool   `json:"enabled"`
	Salt    string `json:"salt"`
	Hash    string `json:"hash"`
}

// Service 锁屏服务。sessionLocked 表示"本会话当前是否被锁定"。
type Service struct {
	mu            sync.Mutex
	path          string
	enabled       bool
	sessionLocked bool
	// keyHex 当前应使用的 SQLCipher 开库 key(hex, 32 字节派生)。
	// 未启用密码 → 默认密码 123456 派生 key(存于 vault.lock); 已启用且已解锁 → 用户密码派生 key; 已启用但锁定 → 空。
	keyHex string
	// currentPassword 当前锁屏密码(明文, 仅本进程内存态, 不落盘):
	// 未启用密码 → 默认密码 123456; 已启用且已解锁 → 用户密码; 已启用但锁定 → 空(禁止导出)。
	currentPassword string
	// rekeyFn 注入数据库 key 切换回调(由 app 注入 store.Rekey)。
	// 设置/修改密码 → rekey 到密码派生 key; 取消密码 → rekey 回默认密码派生 key。
	rekeyFn func(keyHex string) error
}

// New 创建服务(数据目录由 config 确定)。
func New(dataDir string) *Service {
	return &Service{path: filepath.Join(dataDir, "vault.lock")}
}

// SetRekeyFn 注入数据库 key 切换回调(必须在 Init 之前调用)。
func (s *Service) SetRekeyFn(fn func(keyHex string) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rekeyFn = fn
}

// FixedKeyHex 旧版本"未启用锁屏密码"时使用的固定 key(编译期常量派生)。
// 新状态机下该分支已废除(统一用默认密码 123456 派生 key), 此函数仅保留
// 用于老库迁移探测: 启动时新 key 打不开, 则用固定 key 试开, 成功即 rekey 迁移。
func FixedKeyHex() string {
	sum := sha256.Sum256([]byte("LdSSHManager-fixed-key-v1"))
	return hex.EncodeToString(sum[:])
}

// KeyFromPassword 由密码 + salt 派生 SQLCipher key(32 字节 scrypt 输出, hex)。
// 与校验物 Hash 同源: 校验通过即可直接用作开库 key。
func KeyFromPassword(password string, salt []byte) string {
	k, err := scrypt.Key([]byte(password), salt, 16384, 8, 1, 32)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(k)
}

// Init 加载锁屏配置。任何状态下 vault.lock 都保证存在:
//   - 无有效 lock 文件(老版本或全新安装) → 用默认密码 123456 派生并落盘,
//     未启用密码则 keyHex = 默认密码派生 key(可直接开库直进);
//   - 已启用密码则本会话启动即锁定(等待前端显示锁屏遮罩), keyHex 为空。
//
// 同时修正强不变量: enabled=true ⟺ 密码≠默认密码。老数据可能出现
// enabled=true 但密码是默认 123456(用户从未感知的隐蔽密码), 此时视为"未设置",
// 修正落盘并自动直进 —— 否则启动会弹锁屏要求输入用户不知道的密码。
func (s *Service) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lf := s.loadLockFile()
	if lf.Salt == "" || lf.Hash == "" {
		// 无有效 lock 文件: 落盘默认密码 123456 派生 key,
		// 保证"永远有一个锁屏密码"。落盘失败则返回错误, 调用方打日志后应用照常启动。
		salt := randBytes(16)
		h := KeyFromPassword(DefaultPassword, salt)
		if h == "" {
			return errors.New("默认密码派生失败")
		}
		lf = lockFile{Enabled: false, Salt: hex.EncodeToString(salt), Hash: h}
		if err := s.saveLockFile(lf); err != nil {
			return fmt.Errorf("写入默认锁屏配置失败: %w", err)
		}
	}
	// 不变量修正: enabled=true 但密码是默认 123456 → 视为未设置, 直进不锁屏
	if lf.Enabled && s.verifyLocked(DefaultPassword) {
		lf.Enabled = false
		if err := s.saveLockFile(lf); err != nil {
			return fmt.Errorf("修正默认锁屏配置失败: %w", err)
		}
	}
	s.enabled = lf.Enabled
	s.sessionLocked = lf.Enabled
	if s.enabled {
		s.keyHex = ""
		s.currentPassword = ""
	} else {
		// 未启用密码: key = lock 文件中的 hash(默认密码 123456 派生), 启动直接开库。
		s.keyHex = lf.Hash
		s.currentPassword = DefaultPassword
	}
	return nil
}

// CurrentPassword 返回当前用于"导出加密"的密码(内存态, 不落盘):
// 未启用密码 → 默认密码 123456; 已启用且已解锁 → 真实锁屏密码; 已启用但锁定 → 空(调用方应禁止导出)。
func (s *Service) CurrentPassword() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentPassword
}

// KeyHex 当前应使用的 SQLCipher 开库 key; 已启用且锁定(未解锁)时返回空。
func (s *Service) KeyHex() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.keyHex
}

func (s *Service) loadLockFile() lockFile {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return lockFile{}
	}
	var lf lockFile
	_ = json.Unmarshal(raw, &lf)
	return lf
}

func (s *Service) saveLockFile(lf lockFile) error {
	b, err := json.Marshal(lf)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

func checkPwLen(pw string) error {
	n := len([]rune(pw))
	if n < MinLen || n > MaxLen {
		return ErrWrongLength
	}
	return nil
}

// checkPwComposition 锁屏密码必须同时包含字母和数字(用户 08-14 要求);
// 其它字符(符号等)随意, 但字母、数字两者缺一不可。
func checkPwComposition(pw string) error {
	hasLetter, hasDigit := false, false
	for _, r := range pw {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
		if hasLetter && hasDigit {
			return nil
		}
	}
	return ErrWeakPassword
}

// verifyLocked 用落盘的 salt+hash 校验密码(恒定时间比较)。
// 注意: 不检查 lf.Enabled —— 新状态机下 lock 文件永驻, enabled 只控制启动锁屏,
// 密码是否匹配只看 salt+hash 本身。
func (s *Service) verifyLocked(password string) bool {
	lf := s.loadLockFile()
	if lf.Salt == "" || lf.Hash == "" {
		return false
	}
	salt, err1 := hex.DecodeString(lf.Salt)
	want, err2 := hex.DecodeString(lf.Hash)
	if err1 != nil || err2 != nil {
		return false
	}
	got, err := scrypt.Key([]byte(password), salt, 16384, 8, 1, 32)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// verifyCurrent 校验"当前密码": 先尝试默认密码 123456(未设置/已取消状态的当前密码),
// 解不开再校验用户输入。保证任何状态下都有一把可用的钥匙。
// 注意: 仅用于修改/取消密码等管理操作; 锁屏解锁走 Unlock(严格校验)。
func (s *Service) verifyCurrent(pw string) bool {
	if s.verifyLocked(DefaultPassword) {
		return true
	}
	return s.verifyLocked(pw)
}

// ---- 状态 ----

// Status 返回 {ok, locked(本会话是否锁定), enabled(是否已设置密码)}
func (s *Service) Status() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]interface{}{
		"ok":      true,
		"locked":  s.sessionLocked,
		"enabled": s.enabled,
	}
}

// Enabled 是否已设置锁屏密码。
func (s *Service) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

// Locked 本会话是否锁定。
func (s *Service) Locked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessionLocked
}

// Lock 立即锁定本会话(仅当已启用密码时生效)。
func (s *Service) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enabled {
		s.sessionLocked = true
	}
}

// ---- 密码管理 ----

// SetPassword 首次设置锁屏密码: 保存校验物并把数据库 rekey 到密码派生 key。
// 新密码禁止等于默认密码 123456(它是未设置状态的隐蔽密码, 设为它会破坏不变量),
// 且必须同时包含字母和数字。
func (s *Service) SetPassword(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enabled {
		return ErrAlreadyEnabled
	}
	if err := checkPwLen(password); err != nil {
		return err
	}
	if password == DefaultPassword {
		return ErrDefaultPassword
	}
	if err := checkPwComposition(password); err != nil {
		return err
	}
	salt := randBytes(16)
	h := KeyFromPassword(password, salt)
	if h == "" {
		return errors.New("密码派生失败")
	}
	// rekeyFn 必须已注入(app 启动时设置)。若为 nil 直接报错,
	// 避免"设置成功但数据库未切换 key"的静默失败(下次启动锁定后无法开库)。
	if s.rekeyFn == nil {
		return errors.New("数据库 key 切换回调未注入")
	}
	if err := s.rekeyFn(h); err != nil {
		return errors.New("数据库 key 切换失败: " + err.Error())
	}
	if err := s.saveLockFile(lockFile{Enabled: true, Salt: hex.EncodeToString(salt), Hash: h}); err != nil {
		return err
	}
	s.enabled = true
	s.sessionLocked = false // 设置后本会话保持可用; 下次启动需密码
	s.keyHex = h
	s.currentPassword = password
	return nil
}

// ChangePassword 修改锁屏密码(校验旧密码, rekey 到新 key)。
// 旧密码校验: 先试默认密码 123456, 解不开再校验用户输入。
// 新密码禁止等于默认密码 123456(与 SetPassword 同理), 且必须同时包含字母和数字。
func (s *Service) ChangePassword(oldPw, newPw string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled {
		return ErrNoPassword
	}
	if err := checkPwLen(newPw); err != nil {
		return err
	}
	if newPw == DefaultPassword {
		return ErrDefaultPassword
	}
	if err := checkPwComposition(newPw); err != nil {
		return err
	}
	if !s.verifyCurrent(oldPw) {
		return ErrWrongPassword
	}
	salt := randBytes(16)
	h := KeyFromPassword(newPw, salt)
	if h == "" {
		return errors.New("密码派生失败")
	}
	if s.rekeyFn == nil {
		return errors.New("数据库 key 切换回调未注入")
	}
	if err := s.rekeyFn(h); err != nil {
		return errors.New("数据库 key 切换失败: " + err.Error())
	}
	if err := s.saveLockFile(lockFile{Enabled: true, Salt: hex.EncodeToString(salt), Hash: h}); err != nil {
		return err
	}
	s.keyHex = h
	s.currentPassword = newPw
	return nil
}

// DisablePassword 取消锁屏密码 = 重置回默认密码 123456:
// rekey 到默认密码派生 key, vault.lock 保留(enabled=false), 保证"永远有一个锁屏密码"。
// 校验: 先试默认密码, 解不开再校验用户输入。
func (s *Service) DisablePassword(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled {
		return ErrNoPassword
	}
	if !s.verifyCurrent(password) {
		return ErrWrongPassword
	}
	if s.rekeyFn == nil {
		return errors.New("数据库 key 切换回调未注入")
	}
	// 重新派生默认密码 key(新 salt), 落盘后下次启动用该 key 直接开库直进
	salt := randBytes(16)
	h := KeyFromPassword(DefaultPassword, salt)
	if h == "" {
		return errors.New("密码派生失败")
	}
	if err := s.rekeyFn(h); err != nil {
		return errors.New("数据库 key 切换失败: " + err.Error())
	}
	if err := s.saveLockFile(lockFile{Enabled: false, Salt: hex.EncodeToString(salt), Hash: h}); err != nil {
		return err
	}
	s.enabled = false
	s.sessionLocked = false
	s.keyHex = h
	s.currentPassword = DefaultPassword
	return nil
}

// Unlock 解锁(锁屏遮罩入口, 严格校验输入): 校验密码, 成功则本会话解锁。
func (s *Service) Unlock(password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled {
		return ErrNoPassword
	}
	if err := s.unlockLocked(password); err != nil {
		return err
	}
	s.currentPassword = password
	return nil
}

// UnlockWithDefault 解锁(管理操作专用): 先尝试默认密码 123456, 解不开再校验用户输入。
// 用于修改/取消密码等需要旧密码确认的流程; 锁屏解锁请用 Unlock(严格校验)。
func (s *Service) UnlockWithDefault(pw string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled {
		return ErrNoPassword
	}
	if !s.verifyCurrent(pw) {
		return ErrWrongPassword
	}
	// 通过校验: 取实际密码(默认密码优先)派生 key 并解锁
	actual := pw
	if s.verifyLocked(DefaultPassword) {
		actual = DefaultPassword
	}
	if err := s.unlockLocked(actual); err != nil {
		return err
	}
	s.currentPassword = actual
	return nil
}

// unlockLocked 假定已持锁且已通过校验, 用密码派生 key 设置解锁状态。
func (s *Service) unlockLocked(password string) error {
	lf := s.loadLockFile()
	if lf.Salt == "" {
		return ErrWrongPassword
	}
	salt, err1 := hex.DecodeString(lf.Salt)
	want, err2 := hex.DecodeString(lf.Hash)
	if err1 != nil || err2 != nil {
		return ErrWrongPassword
	}
	got, err := scrypt.Key([]byte(password), salt, 16384, 8, 1, 32)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrWrongPassword
	}
	s.sessionLocked = false
	s.keyHex = hex.EncodeToString(got)
	return nil
}
