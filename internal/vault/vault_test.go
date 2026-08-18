package vault

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// rekeyRecorder 记录 rekey 调用(用结构体避免闭包 append 切片的坑)。
type rekeyRecorder struct{ keys []string }

// newTestService 创建使用临时目录的 Service, 注入记录调用的假 rekeyFn。
func newTestService(t *testing.T) (*Service, *rekeyRecorder) {
	t.Helper()
	s := New(t.TempDir())
	r := &rekeyRecorder{}
	s.SetRekeyFn(func(keyHex string) error {
		r.keys = append(r.keys, keyHex)
		return nil
	})
	return s, r
}

// readLockFile 读取落盘的 vault.lock, 校验 JSON 可解析。
func readLockFile(t *testing.T, s *Service) lockFile {
	t.Helper()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("读取 vault.lock 失败: %v", err)
	}
	var lf lockFile
	if err := json.Unmarshal(raw, &lf); err != nil {
		t.Fatalf("解析 vault.lock 失败: %v", err)
	}
	return lf
}

// TestInitCreatesDefaultLock 全新安装: Init 后 lock 文件存在,
// enabled=false 且 keyHex = lock 文件 hash(默认密码派生), 直接开库直进。
func TestInitCreatesDefaultLock(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	lf := readLockFile(t, s)
	if lf.Enabled {
		t.Error("全新状态 enabled 应为 false")
	}
	if lf.Salt == "" || lf.Hash == "" {
		t.Error("lock 文件应含 salt 与 hash")
	}
	if s.KeyHex() != lf.Hash {
		t.Errorf("未启用时 keyHex 应等于 lock 文件 hash, got %q", s.KeyHex())
	}
	if s.Locked() {
		t.Error("未启用时不应锁定")
	}
	// 默认密码派生 key 应与落盘 hash 一致
	if got := KeyFromPassword(DefaultPassword, mustHex(t, lf.Salt)); got != lf.Hash {
		t.Error("默认密码 123456 派生结果应等于落盘 hash")
	}
}

// TestDefaultPasswordAlwaysWorks 未设置状态下, verifyCurrent 用默认密码自动通过。
func TestDefaultPasswordAlwaysWorks(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if !s.verifyCurrent("") {
		t.Error("未设置状态下任意输入都应通过(默认密码优先)")
	}
}

// TestSetPasswordThenDefaultFails 设置密码后: 默认密码 123456 不能再通过,
// 用户密码可通过; lock 文件 enabled=true。
func TestSetPasswordThenDefaultFails(t *testing.T) {
	s, rekeys := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword("abc123"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if !s.Enabled() {
		t.Error("设置后 enabled 应为 true")
	}
	if len(rekeys.keys) != 1 {
		t.Fatalf("SetPassword 应 rekey 1 次, got %d", len(rekeys.keys))
	}
	lf := readLockFile(t, s)
	if !lf.Enabled {
		t.Error("落盘 enabled 应为 true")
	}
	// 默认密码不再有效
	if s.verifyCurrent("") {
		t.Error("设置用户密码后, 默认密码 123456 不应再通过")
	}
	// 用户密码有效
	if !s.verifyCurrent("abc123") {
		t.Error("用户密码应通过校验")
	}
	// 解锁: 123456 拒绝, 用户密码通过
	if err := s.Unlock("123456"); err != ErrWrongPassword {
		t.Errorf("解锁 123456 应报错, got %v", err)
	}
	if err := s.Unlock("abc123"); err != nil {
		t.Errorf("解锁用户密码: %v", err)
	}
	if s.Locked() || s.KeyHex() == "" {
		t.Error("解锁后不应锁定, 且 keyHex 应就绪")
	}
}

// TestChangePassword 修改密码后新密码生效、旧密码失效。
func TestChangePassword(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword("abc123"); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangePassword("abc123", "def456"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if err := s.Unlock("def456"); err != nil {
		t.Errorf("新密码应可解锁: %v", err)
	}
	if err := s.Unlock("abc123"); err != ErrWrongPassword {
		t.Errorf("旧密码应失效, got %v", err)
	}
	// 错误旧密码应拒绝(newPw 仍需合法: 含字母+数字)
	if err := s.ChangePassword("wrong", "abcdef1"); err != ErrWrongPassword {
		t.Errorf("错误旧密码应拒绝, got %v", err)
	}
}

// TestDisablePasswordResetsToDefault 取消密码 = 重置回默认 123456:
// lock 文件保留(enabled=false), keyHex 变为默认密码派生 key。
func TestDisablePasswordResetsToDefault(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword("abc123"); err != nil {
		t.Fatal(err)
	}
	if err := s.DisablePassword("abc123"); err != nil {
		t.Fatalf("DisablePassword: %v", err)
	}
	if s.Enabled() {
		t.Error("取消后 enabled 应为 false")
	}
	if s.Locked() {
		t.Error("取消后不应锁定")
	}
	lf := readLockFile(t, s)
	if lf.Enabled {
		t.Error("落盘 enabled 应为 false")
	}
	if lf.Salt == "" || lf.Hash == "" {
		t.Error("取消后 lock 文件应保留 salt+hash")
	}
	// 默认密码派生 key 应等于落盘 hash(回默认)
	if got := KeyFromPassword(DefaultPassword, mustHex(t, lf.Salt)); got != lf.Hash {
		t.Error("取消后应回到默认密码 123456 派生 key")
	}
	if s.KeyHex() != lf.Hash {
		t.Error("取消后 keyHex 应等于默认密码派生 key")
	}
	// 取消后密码已回到 123456(严格校验): 原用户密码不再匹配, 默认密码匹配
	if s.verifyLocked("abc123") {
		t.Error("取消后原用户密码不应再匹配")
	}
	if !s.verifyLocked(DefaultPassword) {
		t.Error("取消后默认密码应匹配")
	}
	// 管理操作入口 verifyCurrent: 默认密码有效 → 任意输入自动通过
	if !s.verifyCurrent("") {
		t.Error("取消后默认密码应恢复有效")
	}
}

// TestUnlockWithDefault 管理操作解锁: 用户密码设置后, 默认密码解不开, 用户密码可解。
func TestUnlockWithDefault(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword("abc123"); err != nil {
		t.Fatal(err)
	}
	if err := s.UnlockWithDefault(""); err != ErrWrongPassword {
		t.Errorf("默认密码不应解开用户密码, got %v", err)
	}
	if err := s.UnlockWithDefault("abc123"); err != nil {
		t.Errorf("用户密码应可解开: %v", err)
	}
	if !s.Locked() && s.KeyHex() != "" {
		// 正常
	} else {
		t.Error("解锁后 keyHex 应就绪")
	}
}

// TestInitIdempotent Init 重复调用不改变状态(lock 文件已存在时不重新落盘)。
func TestInitIdempotent(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	before := readLockFile(t, s)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	after := readLockFile(t, s)
	if before.Salt != after.Salt || before.Hash != after.Hash {
		t.Error("重复 Init 不应重新生成 salt/hash")
	}
}

// TestInitFixesLegacyEnabledWithDefaultPassword 老数据: enabled=true 但密码是默认 123456
// (用户从未感知的隐蔽密码) → Init 修正为未设置并直进, 不弹锁屏。
func TestInitFixesLegacyEnabledWithDefaultPassword(t *testing.T) {
	s, _ := newTestService(t)
	salt := randBytes(16)
	h := KeyFromPassword(DefaultPassword, salt)
	legacy := lockFile{Enabled: true, Salt: hex.EncodeToString(salt), Hash: h}
	if err := s.saveLockFile(legacy); err != nil {
		t.Fatal(err)
	}
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if s.Enabled() {
		t.Error("密码=默认的老数据应被修正为未设置(enabled=false)")
	}
	if s.Locked() {
		t.Error("密码=默认的老数据不应锁定(避免弹锁屏要求输入隐蔽密码)")
	}
	if s.KeyHex() != h {
		t.Error("修正后 keyHex 应为默认密码派生 key(直进开库)")
	}
	lf := readLockFile(t, s)
	if lf.Enabled {
		t.Error("落盘 enabled 应被修正为 false")
	}
}

// TestSetPasswordRejectsDefault 设置密码禁止使用默认密码 123456。
func TestSetPasswordRejectsDefault(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword(DefaultPassword); err != ErrDefaultPassword {
		t.Errorf("设置 123456 应拒绝, got %v", err)
	}
	if s.Enabled() {
		t.Error("被拒绝后不应启用")
	}
}

// TestChangePasswordRejectsDefault 修改密码禁止把新密码设为默认 123456。
func TestChangePasswordRejectsDefault(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword("abc123"); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangePassword("abc123", DefaultPassword); err != ErrDefaultPassword {
		t.Errorf("新密码 123456 应拒绝, got %v", err)
	}
	if !s.verifyLocked("abc123") {
		t.Error("拒绝后原密码应保持有效")
	}
}

// TestSetPasswordRejectsWeak 密码必须同时包含字母和数字: 纯字母/纯数字都拒绝。
func TestSetPasswordRejectsWeak(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword("abcdef"); err != ErrWeakPassword {
		t.Errorf("纯字母密码应拒绝, got %v", err)
	}
	if err := s.SetPassword("1234567"); err != ErrWeakPassword {
		t.Errorf("纯数字密码应拒绝, got %v", err)
	}
	if s.Enabled() {
		t.Error("被拒绝后不应启用")
	}
	// 字母+数字+符号应通过
	if err := s.SetPassword("a1!@#b2"); err != nil {
		t.Errorf("字母+数字+符号应通过: %v", err)
	}
	if !s.Enabled() {
		t.Error("合法密码后应启用")
	}
}

// TestChangePasswordRejectsWeak 修改密码同样要求新密码含字母+数字。
func TestChangePasswordRejectsWeak(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPassword("abc123"); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangePassword("abc123", "abcdef"); err != ErrWeakPassword {
		t.Errorf("新密码纯字母应拒绝, got %v", err)
	}
	if !s.verifyLocked("abc123") {
		t.Error("拒绝后原密码应保持有效")
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	dec := make([]byte, len(s)/2)
	for i := 0; i < len(dec); i++ {
		hi := hexVal(s[2*i])
		lo := hexVal(s[2*i+1])
		if hi < 0 || lo < 0 {
			t.Fatalf("非法 hex: %q", s)
		}
		dec[i] = byte(hi<<4 | lo)
	}
	return dec
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
