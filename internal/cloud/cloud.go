// Package cloud 云端同步模块 (移植自 LdTools, 适配 LdSSHManager)。
// 负责: 云端账号登录/注册(验证码) + session 配置加密上传/下载 + 自动同步 + 冲突/版本决策。
// 与 LdTools 的差异:
//   - 同步内容为 SSH session 配置(仅文本, 不含 key 文件二进制);
//   - 配置键为 ldsshmanager:sessions, 登录 client 为 ldsshmanager, 与其它工具账号数据隔离;
//   - 云端加密密码 / 登录 token 落盘用 DPAPI 加密(data/cloud_state.json)。
package cloud

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"LdSSHManager/internal/config"
	"LdSSHManager/internal/store"
	"golang.org/x/crypto/scrypt"
)

// 云端配置相关常量
const (
	CloudPwMaxAge  = 7 * 24 * 3600   // 云端加密密码 7 天有效(秒)
	AutoSyncSecs   = 10 * 60         // 自动同步周期 10 分钟
	CloudConfigKey = "ldsshmanager:sessions" // 云端配置 key
	ClientName     = "ldsshmanager"
)

// EncryptedBlob AES-256-GCM 加密结果
type EncryptedBlob struct {
	Salt string `json:"salt"`
	IV   string `json:"iv"`
	Tag  string `json:"tag"`
	Data string `json:"data"`
}

// Service 云端服务
type Service struct {
	mu       sync.RWMutex
	cfg      *Config // 基地址 + 接口路径(从配置文件加载, 不硬编码)
	token    string
	userName string
	nickName string
	deviceID string

	cloudPw         string
	cloudVerifiedAt int64

	// clockOffset 服务器与本地时钟偏移(秒) = 服务器Time - 本地Time, 每次请求响应校准。
	// 用于"谁最新用谁的"自动同步: 本地时间戳换算到服务器时钟再比较, 避免两端时钟差误判。
	clockOffset int64

	httpCli *http.Client
	stopCh  chan struct{}
	emitEvt func(name string, data interface{})
	started bool

	// session 读写钩子(由 app 注入, 避免与 store 循环依赖)
	SessionLoad func() (store.CloudPayload, error)
	SessionSave func(store.CloudPayload) error
}

// New 使用默认配置创建服务
func New() *Service {
	return NewWithConfig(nil)
}

// NewWithConfig 使用指定配置创建服务; cfg==nil 则使用默认配置
func NewWithConfig(cfg *Config) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Service{
		cfg:     cfg,
		httpCli: &http.Client{Timeout: 10 * time.Second},
		stopCh:  make(chan struct{}),
	}
}

// SetEmitEvent 注入事件发射器(由 app 用 wails runtime.EventsEmit 包装)
func (svc *Service) SetEmitEvent(fn func(name string, data interface{})) {
	svc.mu.Lock()
	svc.emitEvt = fn
	svc.mu.Unlock()
}

// SetBaseURL 设置云端 API 基地址(默认走当前配置; env 覆盖)
func (svc *Service) SetBaseURL(url string) {
	if url == "" {
		if env := os.Getenv("LDSSHMANAGER_CLOUD_API"); env != "" {
			url = env
		} else {
			return
		}
	}
	svc.mu.Lock()
	svc.cfg.BaseURL = url
	svc.mu.Unlock()
}

// ---- 持久化 (cloud_state.json) ----

type persistState struct {
	Token              string `json:"token"`
	UserName           string `json:"user_name"`
	NickName           string `json:"nick_name"`
	DeviceID           string `json:"device_id"`
	CloudSyncPw        string `json:"cloud_sync_password"`
	CloudSyncVerifiedAt int64 `json:"cloud_sync_verified_at"`
}

func (svc *Service) statePath() string {
	return filepath.Join(config.DataDir(), "cloud_state.json")
}

// LoadPersist 启动时从本地文件加载登录态与云端加密密码
func (svc *Service) LoadPersist() {
	raw, err := os.ReadFile(svc.statePath())
	if err != nil {
		return
	}
	var p persistState
	if json.Unmarshal(raw, &p) != nil {
		return
	}
	// 敏感字段(云端加密密码/登录 token)落盘为 DPAPI 密文; 兼容旧版明文文件
	token := decryptSecret(p.Token)
	cloudPw := decryptSecret(p.CloudSyncPw)
	svc.mu.Lock()
	svc.token = token
	svc.userName = p.UserName
	svc.nickName = p.NickName
	svc.deviceID = p.DeviceID
	svc.cloudPw = cloudPw
	svc.cloudVerifiedAt = p.CloudSyncVerifiedAt
	svc.mu.Unlock()
	// 旧版明文迁移: 落盘文件里还有明文敏感字段时, 立即用加密格式重写
	if (p.Token != "" && !isEncryptedSecret(p.Token)) ||
		(p.CloudSyncPw != "" && !isEncryptedSecret(p.CloudSyncPw)) {
		svc.savePersist()
	}
}

func (svc *Service) savePersist() {
	svc.mu.RLock()
	p := persistState{
		Token:               encryptSecret(svc.token),
		UserName:            svc.userName,
		NickName:            svc.nickName,
		DeviceID:            svc.deviceID,
		CloudSyncPw:         encryptSecret(svc.cloudPw),
		CloudSyncVerifiedAt: svc.cloudVerifiedAt,
	}
	svc.mu.RUnlock()
	b, _ := json.MarshalIndent(p, "", "  ")
	_ = os.WriteFile(svc.statePath(), b, 0600)
}

// ---- 设备标识 ----

// GetDeviceId 读取(或首次生成)本机持久化设备唯一标识
func (svc *Service) GetDeviceId() string {
	svc.mu.RLock()
	if svc.deviceID != "" {
		id := svc.deviceID
		svc.mu.RUnlock()
		return id
	}
	svc.mu.RUnlock()
	hostPart := fmt.Sprintf("%x", sha1.Sum([]byte(hostname())))[:8]
	randPart := randHex(8)
	id := randPart + "-" + hostPart
	svc.mu.Lock()
	svc.deviceID = id
	svc.mu.Unlock()
	svc.savePersist()
	return id
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "ldsshmanager"
	}
	return h
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---- 状态访问 ----

func (svc *Service) Token() string {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.token
}

func (svc *Service) BaseURL() string {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	if svc.cfg == nil {
		return ""
	}
	return svc.cfg.BaseURL
}

// setClockOffset 记录服务器与本地时钟偏移(秒)
func (svc *Service) setClockOffset(offset int64) {
	svc.mu.Lock()
	svc.clockOffset = offset
	svc.mu.Unlock()
}

// clockOffsetValue 当前服务器时钟偏移(秒)
func (svc *Service) clockOffsetValue() int64 {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.clockOffset
}

// toServerTime 把本地 Unix 秒换算到服务器时钟(与 LdAPI time 同基准, 供时间戳比较)
func (svc *Service) toServerTime(localTs int64) int64 {
	if localTs <= 0 {
		return 0
	}
	return localTs + svc.clockOffsetValue()
}

// Cfg 返回当前配置(只读)
func (svc *Service) Cfg() *Config {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.cfg
}

func (svc *Service) emit(name string, data interface{}) {
	svc.mu.RLock()
	fn := svc.emitEvt
	svc.mu.RUnlock()
	if fn != nil {
		fn(name, data)
	}
}

// ---- 云端加密密码 ----

// SaveCloudSyncPw 保存云端加密密码(明文仅存内存 + DPAPI 加密落盘)
func (svc *Service) SaveCloudSyncPw(pw string) {
	svc.mu.Lock()
	svc.cloudPw = pw
	svc.cloudVerifiedAt = time.Now().Unix()
	svc.mu.Unlock()
	svc.savePersist()
}

// ClearCloudSyncPw 清除云端加密密码本地记录
func (svc *Service) ClearCloudSyncPw() {
	svc.mu.Lock()
	svc.cloudPw = ""
	svc.cloudVerifiedAt = 0
	svc.mu.Unlock()
	svc.savePersist()
}

// CloudPwValid 记录密码是否在 7 天有效期内
func (svc *Service) CloudPwValid() bool {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.cloudPw != "" && svc.cloudVerifiedAt > 0 &&
		time.Now().Unix()-svc.cloudVerifiedAt < CloudPwMaxAge
}

// ---- 加密工具 (AES-256-GCM + scrypt, 与 LdTools/LdRedis 互通) ----

func scryptKey(password string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(password), salt, 16384, 8, 1, 32)
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return b
	}
	return b
}

// EncryptData AES-256-GCM 加密, 返回含 salt/iv/tag/data 的 blob
func EncryptData(plaintext, password string) (*EncryptedBlob, error) {
	salt := randBytes(16)
	key, err := scryptKey(password, salt)
	if err != nil {
		return nil, err
	}
	iv := randBytes(12)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	data := gcm.Seal(nil, iv, []byte(plaintext), nil)
	return &EncryptedBlob{
		Salt: hex.EncodeToString(salt),
		IV:   hex.EncodeToString(iv),
		Tag:  hex.EncodeToString(data[len(data)-16:]),
		Data: hex.EncodeToString(data[:len(data)-16]),
	}, nil
}

// DecryptData AES-256-GCM 解密; 密码错误时返回错误
func DecryptData(blob *EncryptedBlob, password string) (string, error) {
	if blob == nil {
		return "", errors.New("空加密数据")
	}
	salt, err := hex.DecodeString(blob.Salt)
	if err != nil {
		return "", err
	}
	iv, err := hex.DecodeString(blob.IV)
	if err != nil {
		return "", err
	}
	tag, err := hex.DecodeString(blob.Tag)
	if err != nil {
		return "", err
	}
	ct, err := hex.DecodeString(blob.Data)
	if err != nil {
		return "", err
	}
	key, err := scryptKey(password, salt)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	pt, err := gcm.Open(nil, iv, append(ct, tag...), nil)
	if err != nil {
		return "", errors.New("密码错误, 数据无法解密")
	}
	return string(pt), nil
}

// VerifyPassword 恒定时间比较(内部使用)
func VerifyPassword(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ---- LdAPI 请求 ----

// apiRequest 统一请求 {code,msg,time,data}; code===200 成功
func (svc *Service) apiRequest(ctx context.Context, endpoint string, body interface{}, needAuth bool) (interface{}, error) {
	if needAuth && svc.Token() == "" {
		return nil, errors.New("请先登录")
	}
	apiPath := svc.cfg.Endpoint(endpoint)
	if apiPath == "" {
		return nil, fmt.Errorf("未注册接口: %s", endpoint)
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, svc.BaseURL()+apiPath, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if needAuth {
		req.Header.Set("Authorization", "LdToken "+svc.Token())
	}
	resp, err := svc.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接云端服务 %s (%v), 请检查服务端是否运行", svc.BaseURL(), describeNetErr(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("云端服务返回异常状态 HTTP %d", resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.New("云端服务响应异常(返回内容为空或非 JSON), 请检查后端服务")
	}
	var re struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Time int64           `json:"time"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(respBody, &re); err != nil {
		return nil, errors.New("云端服务响应异常(返回内容为空或非 JSON), 请检查后端服务")
	}
	if re.Code != 200 {
		return nil, &APIError{Msg: re.Msg}
	}
	if re.Time > 0 {
		svc.setClockOffset(re.Time - time.Now().Unix())
	}
	if len(re.Data) == 0 || string(re.Data) == "null" {
		return nil, nil
	}
	var data interface{}
	if err := json.Unmarshal(re.Data, &data); err != nil {
		return nil, errors.New("云端服务响应数据解析失败")
	}
	return data, nil
}

// APIError 带错误码的 API 错误
type APIError struct{ Msg string }

func (e *APIError) Error() string { return e.Msg }

func describeNetErr(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "context deadline exceeded"):
		return "连接超时"
	case strings.Contains(s, "connection refused"):
		return "服务端未运行或拒绝连接"
	case strings.Contains(s, "no such host"):
		return "无法解析服务器地址"
	case strings.Contains(s, "connection reset"):
		return "连接被重置"
	}
	return s
}

// ---- 认证 ----

// GetVcode 获取验证码
func (svc *Service) GetVcode(ctx context.Context) (map[string]interface{}, error) {
	apiPath := svc.cfg.Endpoint(EPVcode)
	if apiPath == "" {
		return nil, errors.New("未配置验证码接口")
	}
	baseURL := svc.BaseURL()
	fullURL := strings.TrimRight(baseURL, "/") + apiPath
	log.Printf("[cloud] GET %s", fullURL)

	resp, err := svc.httpCli.Get(fullURL)
	if err != nil {
		log.Printf("[cloud] GET %s 失败: %v", fullURL, describeNetErr(err))
		return nil, err
	}
	defer resp.Body.Close()
	log.Printf("[cloud] GET %s -> HTTP %d", fullURL, resp.StatusCode)

	var re struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&re); err != nil {
		return nil, errors.New("获取验证码失败")
	}
	if re.Code != 200 {
		msg := re.Msg
		if msg == "" {
			msg = "获取验证码失败"
		}
		return nil, errors.New(msg)
	}
	var data map[string]interface{}
	_ = json.Unmarshal(re.Data, &data)
	return data, nil
}

// AuthLogin 登录
func (svc *Service) AuthLogin(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	data, err := svc.apiRequest(ctx, EPLogin, map[string]interface{}{
		"username":  str(params["username"]),
		"password":  str(params["password"]),
		"vcode_id":  str(params["vcodeId"]),
		"vcode_num": str(params["vcodeNum"]),
		"client":    ClientName,
		"device_id": svc.GetDeviceId(),
		"hostname":  hostname(),
	}, false)
	if err != nil {
		return nil, err
	}
	m, _ := data.(map[string]interface{})
	token, _ := m["token"].(string)
	if token == "" {
		return nil, errors.New("登录返回异常")
	}
	info, _ := m["user_info"].(map[string]interface{})
	userName, _ := info["user_name"].(string)
	nickName, _ := info["nick_name"].(string)
	avatarRel, _ := info["avatar"].(string)
	intro, _ := info["intro"].(string)
	email, _ := info["email"].(string)
	mobile, _ := info["mobile"].(string)
	userID := int64(0)
	if v, ok := info["id"].(float64); ok {
		userID = int64(v)
	}
	role := int8(0)
	if v, ok := info["role"].(float64); ok {
		role = int8(v)
	}
	timeReg := int64(0)
	if v, ok := info["time_reg"].(float64); ok {
		timeReg = int64(v)
	}
	timeLogin := int64(0)
	if v, ok := info["time_login"].(float64); ok {
		timeLogin = int64(v)
	}
	// 头像: LdAPI 返回的是相对路径, 前端拼接完整 URL 需 base, 这里后端拼好返回(avatar 为空时给空字符串, 前端用默认头像)
	avatarURL := ""
	if avatarRel != "" {
		avatarURL = "https://file.gxlidang.com/" + avatarRel
	}
	svc.mu.Lock()
	svc.token = token
	svc.userName = userName
	svc.nickName = nickName
	svc.mu.Unlock()
	svc.savePersist()
	return map[string]interface{}{
		"ok":         true,
		"user_name":  userName,
		"nick_name":  nickName,
		"avatar":     avatarURL,
		"intro":      intro,
		"email":      email,
		"mobile":     mobile,
		"id":         userID,
		"role":       role,
		"time_reg":   timeReg,
		"time_login": timeLogin,
	}, nil
}

// AuthRegister 注册
func (svc *Service) AuthRegister(ctx context.Context, params map[string]interface{}) error {
	userName := str(params["user_name"])
	password := str(params["password"])
	nickName := str(params["nick_name"])
	if nickName == "" {
		nickName = userName
	}
	_, err := svc.apiRequest(ctx, EPRegister, map[string]interface{}{
		"user_name": userName,
		"password":  password,
		"nick_name": nickName,
		"vcode_id":  str(params["vcodeId"]),
		"vcode_num": str(params["vcodeNum"]),
	}, false)
	return err
}

// AuthLogout 登出
func (svc *Service) AuthLogout() {
	svc.mu.Lock()
	svc.token = ""
	svc.userName = ""
	svc.nickName = ""
	svc.mu.Unlock()
	svc.ClearCloudSyncPw()
}

// AuthStatus 登录状态
func (svc *Service) AuthStatus() map[string]interface{} {
	if svc.Token() != "" {
		svc.mu.RLock()
		un, nn := svc.userName, svc.nickName
		svc.mu.RUnlock()
		return map[string]interface{}{"ok": true, "loggedIn": true, "user_name": un, "nick_name": nn}
	}
	return map[string]interface{}{"ok": true, "loggedIn": false}
}

// notifyTokenInvalid 通知前端 token 失效(其他设备顶号)
func (svc *Service) notifyTokenInvalid() {
	svc.mu.Lock()
	svc.token = ""
	svc.userName = ""
	svc.nickName = ""
	svc.mu.Unlock()
	svc.ClearCloudSyncPw()
	svc.emit("auto-sync:token-invalid", nil)
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
