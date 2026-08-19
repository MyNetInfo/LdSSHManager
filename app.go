package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"LdSSHManager/internal/cloud"
	"LdSSHManager/internal/config"
	"LdSSHManager/internal/crypto"
	"LdSSHManager/internal/sftpclient"
	"LdSSHManager/internal/sshclient"
	"LdSSHManager/internal/store"
	"LdSSHManager/internal/vault"
	"golang.org/x/crypto/ssh"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
	// Cloud 云端同步服务(session 上传/下载到 LdMain 后端)
	Cloud *cloud.Service
	// Vault 锁屏密码服务(密码派生 SQLCipher 开库 key, 数据库整库加密)
	Vault *vault.Service
	// webviewCleaned 启动前清理掉的残留 WebView2 进程数(>0 时前端弹提示)
	webviewCleaned int
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 锁屏服务: 数据目录由 config 确定。
	a.Vault = vault.New(config.DataDir())
	// 设置/修改/取消锁屏密码时切换数据库 key
	a.Vault.SetRekeyFn(store.Rekey)
	if err := a.Vault.Init(); err != nil {
		fmt.Printf("vault init: %v\n", err)
	}

	// 已启用锁屏密码时, 数据库 key 由密码派生, 启动处于锁定状态 → 不打开 DB,
	// 等待前端锁屏 → 解锁成功后由 VaultUnlock 打开。
	// 未启用时用默认密码派生 key 打开; 若打不开, 尝试旧版本固定 key 并自动迁移。
	if key := a.Vault.KeyHex(); key != "" {
		if err := openStoreWithMigration(a); err != nil {
			fmt.Printf("open store: %v\n", err)
		}
	}

	// 云端配置: 从 data/cloud-config.json 加载(基地址 + 各接口路径),
	// 缺失字段用默认值补齐; 环境变量 LDSSHMANAGER_CLOUD_API 可临时覆盖基地址。
	cloudCfg, err := cloud.LoadConfig(config.DataDir())
	if err != nil {
		fmt.Printf("cloud config load failed: %v (using defaults)\n", err)
	}
	a.Cloud = cloud.NewWithConfig(cloudCfg)
	a.Cloud.SetEmitEvent(func(name string, data interface{}) {
		runtime.EventsEmit(a.ctx, name, data)
	})
	a.Cloud.SessionLoad = func() (store.CloudPayload, error) { return store.LoadCloudPayload() }
	a.Cloud.SessionSave = func(p store.CloudPayload) error { return store.ReplaceAllFromCloud(p) }
	a.Cloud.LoadPersist()
	a.Cloud.StartAutoSync()
}

// domReady is called after the frontend DOM is ready. It emits any deferred
// startup notifications, e.g. telling the user that leftover WebView2 processes
// from a previous abnormal exit were cleaned up so the app could start.
func (a *App) domReady(ctx context.Context) {
	a.ctx = ctx
	if a.webviewCleaned > 0 {
		runtime.EventsEmit(a.ctx, "webview:cleaned", a.webviewCleaned)
	}
}

// Shutdown releases resources when the app exits.
func (a *App) Shutdown(ctx context.Context) {
	for _, id := range sshclient.List() {
		if s := sshclient.Find(id); s != nil {
			s.Close()
		}
	}
	a.Cloud.StopAutoSync()
	_ = store.Close()
}

// GetVersion returns the application version.
func (a *App) GetVersion() string {
	return "v0.8.48"
}

// GetDataDir returns the configured binary data directory.
func (a *App) GetDataDir() string {
	return config.DataDir()
}

// ListSavedSessions returns all saved SSH connection profiles.
// 未解锁(数据库未打开)时返回错误, 前端应处于锁屏状态不会调用。
func (a *App) ListSavedSessions() ([]store.Session, error) {
	if !store.IsOpen() {
		return nil, fmt.Errorf("数据库未打开")
	}
	return store.ListSessions()
}

// SaveSession persists a connection profile.
func (a *App) SaveSession(session store.Session) (store.Session, error) {
	if err := store.SaveSession(&session); err != nil {
		return session, err
	}
	return session, nil
}

// DeleteSession removes a saved profile.
func (a *App) DeleteSession(id int64) error {
	return store.DeleteSession(id)
}

// ===========================================================================
// 快捷命令 (QuickCommand*): 底部快捷命令栏
// ===========================================================================

// ListQuickCommands 返回全部快捷命令(底部快捷命令栏用)。
func (a *App) ListQuickCommands() ([]store.QuickCommand, error) {
	if !store.IsOpen() {
		return nil, fmt.Errorf("数据库未打开")
	}
	return store.ListQuickCommands()
}

// SaveQuickCommand 新增或更新一条快捷命令。
func (a *App) SaveQuickCommand(cmd store.QuickCommand) (store.QuickCommand, error) {
	if !store.IsOpen() {
		return cmd, fmt.Errorf("数据库未打开")
	}
	if err := store.SaveQuickCommand(&cmd); err != nil {
		return cmd, err
	}
	return cmd, nil
}

// DeleteQuickCommand 删除快捷命令。
func (a *App) DeleteQuickCommand(id int64) error {
	if !store.IsOpen() {
		return fmt.Errorf("数据库未打开")
	}
	return store.DeleteQuickCommand(id)
}

// ReorderQuickCommands 重排快捷命令顺序(传入按目标顺序排列的 id 列表)。
func (a *App) ReorderQuickCommands(ids []int64) error {
	if !store.IsOpen() {
		return fmt.Errorf("数据库未打开")
	}
	return store.ReorderQuickCommands(ids)
}

// DuplicateSavedSession creates a copy of a saved profile with a unique name.
func (a *App) DuplicateSavedSession(id int64) (store.Session, error) {
	var empty store.Session
	s, err := store.DuplicateSession(id)
	if err != nil {
		return empty, err
	}
	return *s, nil
}

// GetSetting returns a setting value by key.
func (a *App) GetSetting(key string) (string, error) {
	return store.GetSetting(key)
}

// SetSetting persists a setting value by key.
func (a *App) SetSetting(key, value string) error {
	return store.SetSetting(key, value)
}

// ConnectRequest holds user-provided credentials for a single connect attempt.
type ConnectRequest struct {
	SessionID     int64  `json:"sessionId"`
	Password      string `json:"password"`
	KeyPEM        string `json:"keyPEM"`
	KeyPath       string `json:"keyPath"`
	ProxyPassword string `json:"proxyPassword"` // per-connect only, not persisted
	// Cols/Rows 是前端 xterm 测量的 PTY 初始尺寸(避免 80 列硬编码导致 bash PS1 截断)
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// ConnectResult returns the active session identifier and initial metadata.
type ConnectResult struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Name string `json:"name"`
	// SessionID is the saved profile id this connection belongs to (0 if ad-hoc).
	SessionID int64 `json:"sessionId"`
	// HostKeyPrompt 非空表示连接因未知/变更主机密钥被阻止, 需用户确认后再连。
	HostKeyPrompt *HostKeyPrompt `json:"hostKeyPrompt,omitempty"`
}

// HostKeyPrompt 携带"是否信任此主机"弹窗所需信息。
type HostKeyPrompt struct {
	Host        string `json:"host"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"`
	// Blob 是 base64 marshalled 公钥; 仅 unknown 时填充(供前端经 TrustHostKey 持久化)。
	// changed(密钥变更) 时为空 —— 绝不信任。
	Blob    string `json:"blob"`
	Changed bool   `json:"changed"` // true=密钥已变更(中间人风险), 不信任
}

// HostKeyError 由自定义 HostKeyCallback 在 SSH 握手时返回: 主机密钥不在 known_hosts
// (Kind="unknown", 带指纹与 key blob) 或与已存不一致 (Kind="changed")。
type HostKeyError struct {
	Kind        string
	Host        string
	KeyType     string
	Fingerprint string
	Blob        string
}

func (e *HostKeyError) Error() string {
	if e.Kind == "changed" {
		return fmt.Sprintf("host key changed (host=%s type=%s fp=%s): possible MITM", e.Host, e.KeyType, e.Fingerprint)
	}
	return fmt.Sprintf("unknown host key (host=%s type=%s fp=%s)", e.Host, e.KeyType, e.Fingerprint)
}

// ConnectSSH opens an SSH session and starts the terminal/SFTP pumps.
func (a *App) ConnectSSH(req ConnectRequest) (ConnectResult, error) {
	var empty ConnectResult
	saved, err := store.GetSession(req.SessionID)
	if err != nil {
		return empty, fmt.Errorf("load session: %w", err)
	}

	password := req.Password
	if password == "" {
		password = saved.Password
	}

	opts := sshclient.ConnectOptions{
		Name:          saved.Name,
		Host:          saved.Host,
		Port:          saved.Port,
		Username:      saved.Username,
		Password:      password,
		ProxyType:     saved.ProxyType,
		ProxyHost:     saved.ProxyHost,
		ProxyPort:     saved.ProxyPort,
		ProxyUsername: saved.ProxyUsername,
		ProxyPassword: req.ProxyPassword,
		ExecCmd:       saved.ExecCmd,
		SavedID:       saved.ID,
		Cols:          req.Cols,
		Rows:          req.Rows,
		// known_hosts 式主机密钥校验(首次未知→弹窗确认; 变更→阻止)
		HostKeyCallback: a.hostKeyCallback(),
	}
	if saved.AuthType == "key" {
		if req.KeyPEM != "" {
			opts.PrivateKey = req.KeyPEM
		} else if req.KeyPath != "" {
			data, err := os.ReadFile(req.KeyPath)
			if err != nil {
				return empty, fmt.Errorf("read key file: %w", err)
			}
			opts.PrivateKey = string(data)
		}
	}

	sess, err := sshclient.Connect(opts,
		func(id string, data []byte) {
			runtime.EventsEmit(a.ctx, "ssh:data", map[string]string{
				"id":   id,
				"data": base64.StdEncoding.EncodeToString(data),
			})
		},
		func(id string, reason string) {
			runtime.EventsEmit(a.ctx, "ssh:close", map[string]string{
				"id":     id,
				"reason": reason,
			})
		},
	)
	if err != nil {
		var hk *HostKeyError
		if errors.As(err, &hk) {
			return ConnectResult{
				Host:      saved.Host,
				Name:      saved.Name,
				SessionID: saved.ID,
				HostKeyPrompt: &HostKeyPrompt{
					Host:        hk.Host,
					KeyType:     hk.KeyType,
					Fingerprint: hk.Fingerprint,
					Blob:        hk.Blob,
					Changed:     hk.Kind == "changed",
				},
			}, nil
		}
		return empty, err
	}

	runtime.EventsEmit(a.ctx, "ssh:open", map[string]string{
		"id":   sess.ID,
		"host": saved.Host,
		"name": saved.Name,
	})

	return ConnectResult{
		ID:        sess.ID,
		Host:      saved.Host,
		Name:      saved.Name,
		SessionID: saved.ID,
	}, nil
}

// hostKeyCallback 返回 known_hosts 风格的 ssh.HostKeyCallback: 查 known_hosts 表,
// 未知主机返回 *HostKeyError(Kind=unknown, 带指纹+key blob 供前端持久化),
// 密钥变更返回 *HostKeyError(Kind=changed, 绝不自动信任), 已知且匹配返回 nil。
func (a *App) hostKeyCallback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		host := hostname
		keyType := key.Type()
		blob := base64.StdEncoding.EncodeToString(key.Marshal())
		known, err := store.GetKnownHost(host, keyType)
		if err != nil {
			return fmt.Errorf("known hosts lookup: %w", err)
		}
		if known == "" {
			return &HostKeyError{
				Kind:        "unknown",
				Host:        host,
				KeyType:     keyType,
				Fingerprint: ssh.FingerprintSHA256(key),
				Blob:        blob,
			}
		}
		if known != blob {
			return &HostKeyError{
				Kind:        "changed",
				Host:        host,
				KeyType:     keyType,
				Fingerprint: ssh.FingerprintSHA256(key),
			}
		}
		return nil
	}
}

// TrustHostKey 把用户显式信任的主机密钥写入 known_hosts, 后续同主机连接不再弹窗。
func (a *App) TrustHostKey(host, keyType, blob string) error {
	return store.AddKnownHost(host, keyType, blob)
}

// ResetHostKey 删除指定主机的全部已信任密钥记录(known_hosts)。
// 场景: 远端主机重装系统后密钥已更换, 连接被"密钥已变更"防护阻止;
// 用户确认该主机无误后调用本方法重置本地记录, 下次连接回到"未知主机"
// 指纹确认流程(仍不绕过安全确认)。
func (a *App) ResetHostKey(host string) error {
	return store.DeleteKnownHost(host)
}

// DuplicateSSH creates a new active session using the same options as an existing one.
func (a *App) DuplicateSSH(id string) (ConnectResult, error) {
	var empty ConnectResult
	sess, err := sshclient.Duplicate(id,
		func(id string, data []byte) {
			runtime.EventsEmit(a.ctx, "ssh:data", map[string]string{
				"id":   id,
				"data": base64.StdEncoding.EncodeToString(data),
			})
		},
		func(id string, reason string) {
			runtime.EventsEmit(a.ctx, "ssh:close", map[string]string{
				"id":     id,
				"reason": reason,
			})
		},
	)
	if err != nil {
		var hk *HostKeyError
		if errors.As(err, &hk) {
			return ConnectResult{
				Host:      sess.Opts.Host,
				Name:      sess.Opts.Name,
				SessionID: sess.Opts.SavedID,
				HostKeyPrompt: &HostKeyPrompt{
					Host:        hk.Host,
					KeyType:     hk.KeyType,
					Fingerprint: hk.Fingerprint,
					Blob:        hk.Blob,
					Changed:     hk.Kind == "changed",
				},
			}, nil
		}
		return empty, err
	}

	runtime.EventsEmit(a.ctx, "ssh:open", map[string]string{
		"id":   sess.ID,
		"host": sess.Opts.Host,
		"name": sess.Opts.Name,
	})

	return ConnectResult{
		ID:        sess.ID,
		Host:      sess.Opts.Host,
		Name:      sess.Opts.Name,
		SessionID: sess.Opts.SavedID,
	}, nil
}

// DisconnectSSH closes an active SSH session.
func (a *App) DisconnectSSH(id string) {
	if s := sshclient.Find(id); s != nil {
		s.Close()
	}
}

// SSHSessionResize 调整指定会话的 PTY 行列(前端 xterm fit 后同步, 保证 bash 排版正确)。
func (a *App) SSHSessionResize(id string, cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("invalid size %dx%d", cols, rows)
	}
	s := sshclient.Find(id)
	if s == nil {
		return fmt.Errorf("session %s not found", id)
	}
	return s.Resize(cols, rows)
}

// SSHSendData sends keyboard input to the remote shell.
func (a *App) SSHSendData(id string, data string) error {
	s := sshclient.Find(id)
	if s == nil {
		return fmt.Errorf("session %s not found", id)
	}
	return s.Send([]byte(data))
}

// SFTPListResult holds the current path and its entries.
type SFTPListResult struct {
	Path    string             `json:"path"`
	Entries []sftpclient.Entry `json:"entries"`
}

// SFTPList lists a remote directory for the active SSH session.
func (a *App) SFTPList(sessionID string, path string) (SFTPListResult, error) {
	var empty SFTPListResult
	s := sshclient.Find(sessionID)
	if s == nil {
		return empty, fmt.Errorf("session %s not active", sessionID)
	}
	client, err := s.GetSFTP()
	if err != nil {
		return empty, err
	}
	entries, err := sftpclient.List(client, path)
	if err != nil {
		return empty, err
	}
	return SFTPListResult{Path: path, Entries: entries}, nil
}

// SFTPUpload uploads a local file to a remote path.
func (a *App) SFTPUpload(sessionID, localPath, remotePath string) error {
	s := sshclient.Find(sessionID)
	if s == nil {
		return fmt.Errorf("session %s not active", sessionID)
	}
	client, err := s.GetSFTP()
	if err != nil {
		return err
	}
	return sftpclient.Upload(client, localPath, remotePath)
}

// SFTPDownload downloads a remote file to a local path.
func (a *App) SFTPDownload(sessionID, remotePath, localPath string) error {
	s := sshclient.Find(sessionID)
	if s == nil {
		return fmt.Errorf("session %s not active", sessionID)
	}
	client, err := s.GetSFTP()
	if err != nil {
		return err
	}
	return sftpclient.Download(client, remotePath, localPath)
}

// SFTPRename renames a remote path.
func (a *App) SFTPRename(sessionID, oldPath, newPath string) error {
	s := sshclient.Find(sessionID)
	if s == nil {
		return fmt.Errorf("session %s not active", sessionID)
	}
	client, err := s.GetSFTP()
	if err != nil {
		return err
	}
	return sftpclient.Rename(client, oldPath, newPath)
}

// SFTPDelete deletes a remote file or directory.
func (a *App) SFTPDelete(sessionID, path string) error {
	s := sshclient.Find(sessionID)
	if s == nil {
		return fmt.Errorf("session %s not active", sessionID)
	}
	client, err := s.GetSFTP()
	if err != nil {
		return err
	}
	return sftpclient.Delete(client, path)
}

// SFTPMkdir creates a remote directory.
func (a *App) SFTPMkdir(sessionID, path string) error {
	s := sshclient.Find(sessionID)
	if s == nil {
		return fmt.Errorf("session %s not active", sessionID)
	}
	client, err := s.GetSFTP()
	if err != nil {
		return err
	}
	return sftpclient.Mkdir(client, path)
}

// PickFile opens a native file picker and returns the chosen path ("" if cancelled).
func (a *App) PickFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select file",
		Filters: []runtime.FileFilter{
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
}

// PickSavePath opens a native save dialog with the given default name.
func (a *App) PickSavePath(defaultName string) (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save as",
		DefaultFilename: defaultName,
	})
}

// ===========================================================================
// 导出 / 导入 (加密备份: 会话列表 + 快捷命令)
// ===========================================================================

// backupPayload 是加密备份内部明文结构。
type backupPayload struct {
	Schema        int                 `json:"schema"`
	App           string              `json:"app"`
	Exported      string              `json:"exported"`
	Sessions      []store.Session     `json:"sessions"`
	QuickCommands []store.QuickCommand `json:"quickCommands"`
}

// ExportData 导出全部 会话 + 快捷命令, 用"当前锁屏密码"加密后写入 .ldssh 文件。
// 未设置锁屏密码时自动用默认密码 123456 加密(usedDefault=true 让前端区分提醒文案)。
// 锁屏状态下(currentPassword 为空)禁止导出。
// 返回 {ok, filePath, count, qcCount, usedDefault} 或 {ok:false, error} 或 {ok:true, canceled:true}。
func (a *App) ExportData() map[string]interface{} {
	if !store.IsOpen() {
		return map[string]interface{}{"ok": false, "error": "数据库未打开, 请先解锁"}
	}
	if a.Vault == nil {
		return map[string]interface{}{"ok": false, "error": "锁屏服务未初始化"}
	}
	pw := a.Vault.CurrentPassword()
	if pw == "" {
		return map[string]interface{}{"ok": false, "error": "请先解锁后再导出"}
	}
	sessions, err := store.ListSessions()
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	qcs, err := store.ListQuickCommands()
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	payload := backupPayload{
		Schema:        2,
		App:           "LdSSHManager",
		Exported:      time.Now().Format("2006-01-02 15:04:05"),
		Sessions:      sessions,
		QuickCommands: qcs,
	}
	plain, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	usedDefault := pw == vault.DefaultPassword
	enc, err := crypto.EncryptString(string(plain), pw)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	defaultName := "LdSSHManager-Backup-" + time.Now().Format("2006-01-02") + ".ldssh"
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出数据 (已加密)",
		DefaultFilename: defaultName,
		Filters:         []runtime.FileFilter{{DisplayName: "LdSSHManager 备份", Pattern: "*.ldssh"}},
	})
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	if filePath == "" {
		return map[string]interface{}{"ok": true, "canceled": true}
	}
	if err := os.WriteFile(filePath, []byte(enc), 0o600); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{
		"ok":          true,
		"filePath":    filePath,
		"count":       len(sessions),
		"qcCount":     len(qcs),
		"usedDefault": usedDefault,
	}
}

// decryptBackup 读文件并用给定密码解密, 解析为 会话 + 快捷命令。
func (a *App) decryptBackup(filePath, password string) (*backupPayload, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	plain, err := crypto.DecryptString(string(raw), password)
	if err != nil {
		return nil, err
	}
	var payload backupPayload
	if err := json.Unmarshal([]byte(plain), &payload); err != nil {
		return nil, errors.New("备份内容解析失败 (可能密码错误)")
	}
	if payload.Sessions == nil {
		payload.Sessions = []store.Session{}
	}
	if payload.QuickCommands == nil {
		payload.QuickCommands = []store.QuickCommand{}
	}
	return &payload, nil
}

// ImportData 选择加密备份文件: 先用默认密码 123456 试解密;
// 成功则返回待确认数据(needPassword=false); 失败(用户设过锁屏密码)返回 needPassword=true 让前端弹密码框。
func (a *App) ImportData() map[string]interface{} {
	if !store.IsOpen() {
		return map[string]interface{}{"ok": false, "error": "数据库未打开"}
	}
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                "导入数据 (加密备份)",
		Filters:              []runtime.FileFilter{{DisplayName: "LdSSHManager 备份", Pattern: "*.ldssh"}},
		CanCreateDirectories: false,
	})
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	if filePath == "" {
		return map[string]interface{}{"ok": true, "canceled": true}
	}
	// 先试默认密码 123456
	if p, e := a.decryptBackup(filePath, vault.DefaultPassword); e == nil {
		return map[string]interface{}{
			"ok":       true,
			"filePath": filePath,
			"count":    len(p.Sessions),
			"qcCount":  len(p.QuickCommands),
			"sessions": p.Sessions,
			"quickCommands": p.QuickCommands,
		}
	}
	// 默认密码打不开 → 需要用户输入备份时的锁屏密码
	return map[string]interface{}{"ok": true, "needPassword": true, "filePath": filePath}
}

// ImportDataWithPassword 用用户提供的密码解密指定备份文件。
// 成功返回待确认数据; 失败(password 错/损坏)返回 {ok:false, error}。
func (a *App) ImportDataWithPassword(filePath, password string) map[string]interface{} {
	if !store.IsOpen() {
		return map[string]interface{}{"ok": false, "error": "数据库未打开"}
	}
	p, err := a.decryptBackup(filePath, password)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{
		"ok":       true,
		"filePath": filePath,
		"count":    len(p.Sessions),
		"qcCount":  len(p.QuickCommands),
		"sessions": p.Sessions,
		"quickCommands": p.QuickCommands,
	}
}

// ApplyImport 用导入的 会话 + 快捷命令 全量覆盖本地(前端确认后调用)。
func (a *App) ApplyImport(sessions []store.Session, quickCommands []store.QuickCommand) (map[string]interface{}, error) {
	if !store.IsOpen() {
		return map[string]interface{}{"ok": false, "error": "数据库未打开"}, nil
	}
	if sessions == nil {
		sessions = []store.Session{}
	}
	if quickCommands == nil {
		quickCommands = []store.QuickCommand{}
	}
	if err := store.ReplaceAllSessions(sessions); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}, nil
	}
	if err := store.ReplaceAllQuickCommands(quickCommands); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}, nil
	}
	return map[string]interface{}{"ok": true, "count": len(sessions), "qcCount": len(quickCommands)}, nil
}

// ===========================================================================
// 锁屏密码保险库 (Vault*)
// ===========================================================================

// VaultStatus 锁屏状态: {ok, locked(本会话是否锁定), enabled(是否已设置密码)}
func (a *App) VaultStatus() map[string]interface{} {
	if a.Vault == nil {
		return map[string]interface{}{"ok": false, "locked": false, "enabled": false}
	}
	return a.Vault.Status()
}

// VaultSetPassword 首次设置锁屏密码(校验后 rekey 数据库到密码派生 key)
func (a *App) VaultSetPassword(password string) map[string]interface{} {
	if a.Vault == nil {
		return map[string]interface{}{"ok": false, "error": "锁屏服务未初始化"}
	}
	if err := a.Vault.SetPassword(password); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

// VaultChangePassword 修改锁屏密码(校验旧密码, rekey 到新 key)。
// 锁定时数据库已关闭: 先用旧密码解锁并打开数据库, 才能执行 rekey。
func (a *App) VaultChangePassword(oldPassword, newPassword string) map[string]interface{} {
	if a.Vault == nil {
		return map[string]interface{}{"ok": false, "error": "锁屏服务未初始化"}
	}
	// 校验旧密码并派生 key(解锁); 失败则不继续。
	// 先尝试默认密码 123456, 解不开再校验用户输入。
	if err := a.Vault.UnlockWithDefault(oldPassword); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	if err := openDBIfNeeded(a); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	if err := a.Vault.ChangePassword(oldPassword, newPassword); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

// VaultDisablePassword 取消锁屏密码 = 重置回默认密码 123456。
// 锁定时数据库已关闭: 先用密码解锁并打开数据库, 才能执行 rekey。
func (a *App) VaultDisablePassword(password string) map[string]interface{} {
	if a.Vault == nil {
		return map[string]interface{}{"ok": false, "error": "锁屏服务未初始化"}
	}
	// 先尝试默认密码 123456, 解不开再校验用户输入
	if err := a.Vault.UnlockWithDefault(password); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	if err := openDBIfNeeded(a); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	if err := a.Vault.DisablePassword(password); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	return map[string]interface{}{"ok": true}
}

// openStoreWithMigration 用当前 vault key 打开数据库;
// 若新 key 打不开, 尝试旧版本"未设置密码"留下的固定 key, 成功则 rekey 迁移到新 key。
// 覆盖场景: 全新安装 / 老库固定 key 加密 / 已设置密码(锁定态 key 为空直接跳过)。
func openStoreWithMigration(a *App) error {
	key := a.Vault.KeyHex()
	if key == "" {
		return nil // 已启用密码且锁定, 等解锁后再打开
	}
	if err := store.Open(key); err == nil {
		return nil
	}
	// 新 key 打不开: 可能是旧版本固定 key 加密的老库, 尝试迁移
	if err := store.Open(vault.FixedKeyHex()); err != nil {
		return err // 两边都打不开: 交给调用方打日志
	}
	if err := store.Rekey(key); err != nil {
		return fmt.Errorf("migrate legacy encrypted db: %w", err)
	}
	return nil
}

// openDBIfNeeded 数据库未打开时用当前派生 key 打开(修改/取消密码前的 rekey 前置)。
func openDBIfNeeded(a *App) error {
	if store.IsOpen() {
		return nil
	}
	key := a.Vault.KeyHex()
	if key == "" {
		return fmt.Errorf("无法派生数据库 key")
	}
	return store.Open(key)
}

// VaultUnlock 解锁: 校验密码, 成功则本会话解锁并用密码派生 key 打开数据库。
func (a *App) VaultUnlock(password string) map[string]interface{} {
	if a.Vault == nil {
		return map[string]interface{}{"ok": false, "error": "锁屏服务未初始化"}
	}
	if err := a.Vault.Unlock(password); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	if err := openDBIfNeeded(a); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	// 通知前端: 数据已就绪, 可重新加载
	runtime.EventsEmit(a.ctx, "vault:unlocked", nil)
	return map[string]interface{}{"ok": true}
}

// VaultLock 立即锁定本会话(仅已启用密码时生效)。
// 锁定同时关闭数据库句柄, 未解锁前数据不可读。
func (a *App) VaultLock() map[string]interface{} {
	if a.Vault == nil {
		return map[string]interface{}{"ok": false, "error": "锁屏服务未初始化"}
	}
	a.Vault.Lock()
	if store.IsOpen() {
		_ = store.Close()
	}
	return map[string]interface{}{"ok": true}
}

// ===========================================================================
// 服务器状态采集 (SessionStats)
// ===========================================================================

// SessionStats 采集指定会话的服务器实时状态(CPU/内存/磁盘/线程/负载/运行时间)。
// 内部通过独立 SSH exec channel 并发执行远程命令, 不污染交互终端。
// 会话未连接或不存在时返回 error(前端据此显示提示)。
func (a *App) SessionStats(sessionId string) (*sshclient.Stats, error) {
	sess := sshclient.Find(sessionId)
	if sess == nil {
		return nil, fmt.Errorf("会话不存在或已断开")
	}
	return sess.CollectStats()
}

// ===========================================================================
// 云端同步 (Cloud*)
// ===========================================================================

// CloudAuthGetVcode 获取登录验证码
func (a *App) CloudAuthGetVcode() (map[string]interface{}, error) {
	if a.Cloud == nil {
		return nil, nil
	}
	return a.Cloud.GetVcode(a.ctx)
}

// CloudAuthLogin 登录云端账号
func (a *App) CloudAuthLogin(username, password, vcodeID, vcodeNum string) (map[string]interface{}, error) {
	if a.Cloud == nil {
		return map[string]interface{}{"ok": false, "error": "云端未初始化"}, nil
	}
	return a.Cloud.AuthLogin(a.ctx, map[string]interface{}{
		"username": username,
		"password": password,
		"vcodeId":  vcodeID,
		"vcodeNum": vcodeNum,
	})
}

// CloudAuthRegister 注册云端账号
func (a *App) CloudAuthRegister(username, password, nickName, vcodeID, vcodeNum string) error {
	if a.Cloud == nil {
		return nil
	}
	return a.Cloud.AuthRegister(a.ctx, map[string]interface{}{
		"user_name": username,
		"password":  password,
		"nick_name": nickName,
		"vcodeId":   vcodeID,
		"vcodeNum":  vcodeNum,
	})
}

// CloudAuthLogout 登出云端账号
func (a *App) CloudAuthLogout() {
	if a.Cloud != nil {
		a.Cloud.AuthLogout()
	}
}

// CloudAuthStatus 云端登录状态
func (a *App) CloudAuthStatus() map[string]interface{} {
	if a.Cloud == nil {
		return map[string]interface{}{"ok": true, "loggedIn": false}
	}
	return a.Cloud.AuthStatus()
}

// CloudUploadConfig 上传 session 到云端(仅文本, 加密)
func (a *App) CloudUploadConfig(password string, reset, force bool) (map[string]interface{}, error) {
	if a.Cloud == nil {
		return map[string]interface{}{"ok": false, "error": "云端未初始化"}, nil
	}
	return a.Cloud.UploadConfig(a.ctx, password, reset, force)
}

// CloudListVersions 列出云端 session 版本
func (a *App) CloudListVersions() (map[string]interface{}, error) {
	if a.Cloud == nil {
		return map[string]interface{}{"ok": false, "error": "云端未初始化"}, nil
	}
	return a.Cloud.ListVersions(a.ctx)
}

// CloudDownloadConfig 下载指定版本(解密后不落盘)
func (a *App) CloudDownloadConfig(version int, password string) (map[string]interface{}, error) {
	if a.Cloud == nil {
		return map[string]interface{}{"ok": false, "error": "云端未初始化"}, nil
	}
	return a.Cloud.DownloadConfig(a.ctx, version, password)
}

// CloudApplyDownloaded 用云端 session 覆盖本地(已解密数据)
func (a *App) CloudApplyDownloaded(payload store.CloudPayload) (map[string]interface{}, error) {
	if a.Cloud == nil {
		return map[string]interface{}{"ok": false, "error": "云端未初始化"}, nil
	}
	return a.Cloud.ApplyDownloaded(payload)
}

// CloudAutoSyncIgnore 自动同步-解密失败选"忽略并停止"
func (a *App) CloudAutoSyncIgnore() map[string]interface{} {
	if a.Cloud == nil {
		return map[string]interface{}{"ok": true}
	}
	return a.Cloud.AutoSyncIgnore()
}

// CloudAutoSyncDismiss 自动同步-冲突选"暂不处理"
func (a *App) CloudAutoSyncDismiss() map[string]interface{} {
	if a.Cloud == nil {
		return map[string]interface{}{"ok": true}
	}
	return a.Cloud.AutoSyncDismiss()
}

// CloudAutoSyncRetry 自动同步-解密失败选"用新密码重试"
func (a *App) CloudAutoSyncRetry(password string) (map[string]interface{}, error) {
	if a.Cloud == nil {
		return map[string]interface{}{"ok": false, "error": "云端未初始化"}, nil
	}
	return a.Cloud.AutoSyncRetryWithPassword(a.ctx, password)
}
