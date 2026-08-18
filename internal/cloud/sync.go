package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"LdSSHManager/internal/store"
)

// ===========================================================================
// 云端自动同步
// ===========================================================================

// StartAutoSync 条件满足(已登录+有加密密码)则每 10 分钟自动同步
func (svc *Service) StartAutoSync() {
	svc.mu.Lock()
	if svc.started {
		svc.mu.Unlock()
		return
	}
	svc.started = true
	svc.stopCh = make(chan struct{})
	svc.mu.Unlock()
	go func() {
		svc.autoSyncOnce()
		ticker := time.NewTicker(AutoSyncSecs * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-svc.stopCh:
				return
			case <-ticker.C:
				svc.autoSyncOnce()
			}
		}
	}()
}

// StopAutoSync 停止自动同步
func (svc *Service) StopAutoSync() {
	svc.mu.Lock()
	if !svc.started {
		svc.mu.Unlock()
		return
	}
	svc.started = false
	select {
	case <-svc.stopCh:
	default:
		close(svc.stopCh)
	}
	svc.mu.Unlock()
}

// notifyPasswordInvalid 通知前端: 加密密码失效, 自动同步已停止
func (svc *Service) notifyPasswordInvalid(message string) {
	svc.ClearCloudSyncPw()
	svc.emit("auto-sync:password-invalid", message)
}

// ClearSyncPending 解除等待用户决策标志(兼容保留)
func (svc *Service) ClearSyncPending() {}

// buildCloudPayload 构建上传 payload(仅文本: 会话 + 快捷命令)
func (svc *Service) buildCloudPayload() (store.CloudPayload, error) {
	if svc.SessionLoad == nil {
		return store.CloudPayload{Schema: 2, Sessions: []store.CloudSession{}, QuickCommands: []store.CloudQuickCommand{}}, nil
	}
	p, err := svc.SessionLoad()
	if err != nil {
		return store.CloudPayload{}, err
	}
	if p.Sessions == nil {
		p.Sessions = []store.CloudSession{}
	}
	if p.QuickCommands == nil {
		p.QuickCommands = []store.CloudQuickCommand{}
	}
	p.Schema = 2
	return p, nil
}

// canonicalPayload 规范化(用于比较/判等; 不含时间戳)
func canonicalPayload(p store.CloudPayload) string {
	out, _ := json.Marshal(map[string]interface{}{
		"schema":        p.Schema,
		"sessions":      p.Sessions,
		"quickCommands": p.QuickCommands,
	})
	return string(out)
}

// compareRemote 拉取服务器最新版本并解密, 与本地明文对比
// 返回 'same' | 'diff' | 'noData'; 解密失败抛错
func (svc *Service) compareRemote(ctx context.Context, plain, pw string) (string, error) {
	res, err := svc.apiRequest(ctx, EPConfigGet, map[string]interface{}{"key": CloudConfigKey}, true)
	if err != nil {
		return "", err
	}
	m, _ := res.(map[string]interface{})
	if m == nil || m["data"] == nil {
		return "noData", nil
	}
	var blob EncryptedBlob
	dataStr, _ := m["data"].(string)
	if err := json.Unmarshal([]byte(dataStr), &blob); err != nil {
		return "", errors.New("解密失败: 云端数据无法用该密码解密(可能已在其他电脑更换)")
	}
	remotePlain, err := DecryptData(&blob, pw)
	if err != nil {
		return "", errors.New("解密失败: 云端数据无法用该密码解密(可能已在其他电脑更换)")
	}
	if canonicalPayload(mustPayload(remotePlain)) == plain {
		return "same", nil
	}
	return "diff", nil
}

func mustPayload(plain string) store.CloudPayload {
	var p store.CloudPayload
	_ = json.Unmarshal([]byte(plain), &p)
	return p
}

// autoSyncOnce 自动同步一次: 规则为"谁最新用谁的"(last-write-wins), 不再弹窗确认。
func (svc *Service) autoSyncOnce() {
	if svc.Token() == "" {
		return // 未登录不自动同步
	}
	if !svc.CloudPwValid() {
		svc.StopAutoSync()
		svc.notifyPasswordInvalid("云端加密密码已超过 7 天未验证, 自动同步已停止; 需重新上传一次输入密码以恢复")
		return
	}
	plain, err := svc.buildCloudPayload()
	if err != nil {
		return
	}
	plainStr := canonicalPayload(plain)
	ctx := context.Background()
	res, err := svc.apiRequest(ctx, EPConfigGet, map[string]interface{}{"key": CloudConfigKey}, true)
	if err != nil {
		if strings.Contains(err.Error(), "其他设备") {
			svc.notifyTokenInvalid()
		}
		return // 网络等其他错误: 静默, 下个周期重试
	}
	m, _ := res.(map[string]interface{})
	if m == nil || m["data"] == nil {
		// 远程无数据: 直接上传本地作为版本1
		if svc.CloudPwValid() {
			blob, _ := EncryptData(plainStr, svc.currentCloudPw())
			blobJSON, _ := json.Marshal(blob)
			_, _ = svc.apiRequest(ctx, EPConfigSave, map[string]interface{}{
				"key": CloudConfigKey, "data": string(blobJSON),
			}, true)
		}
		return
	}
	var blob EncryptedBlob
	dataStr, _ := m["data"].(string)
	if err := json.Unmarshal([]byte(dataStr), &blob); err != nil {
		svc.StopAutoSync()
		svc.notifyPasswordInvalid("云端加密密码无法解密历史版本(可能已在其他电脑更换), 自动同步已停止; 可手动上传/下载时选择处理")
		return
	}
	remotePlain, err := DecryptData(&blob, svc.currentCloudPw())
	if err != nil {
		svc.StopAutoSync()
		svc.notifyPasswordInvalid("云端加密密码无法解密历史版本(可能已在其他电脑更换), 自动同步已停止; 可手动上传/下载时选择处理")
		return
	}
	if canonicalPayload(mustPayload(remotePlain)) == plainStr {
		return // 一致: 什么都不用做
	}
	// 内容不一致: 谁最新用谁的
	remoteTs := int64(0)
	if v, ok := m["time_create"].(float64); ok {
		remoteTs = int64(v)
	}
	localTs := svc.localUpdatedAtServer() // 本地最后修改, 已换算服务器时钟
	if remoteTs > 0 && localTs > 0 && remoteTs > localTs {
		// 远程更新: 下载覆盖本地
		if err := svc.applyRemotePayload(mustPayload(remotePlain)); err != nil {
			log.Printf("[cloud] auto-sync 远程覆盖本地失败: %v", err)
			return
		}
		log.Printf("[cloud] auto-sync: 远程更新(remote=%d > local=%d), 已下载覆盖本地", remoteTs, localTs)
		return
	}
	// 本地更新或时间无法判断: 上传本地覆盖远程
	blob2, _ := EncryptData(plainStr, svc.currentCloudPw())
	blobJSON2, _ := json.Marshal(blob2)
	if _, err := svc.apiRequest(ctx, EPConfigSave, map[string]interface{}{
		"key": CloudConfigKey, "data": string(blobJSON2),
	}, true); err != nil {
		log.Printf("[cloud] auto-sync 本地上传覆盖远程失败: %v", err)
		return
	}
	log.Printf("[cloud] auto-sync: 本地更新(local=%d >= remote=%d), 已上传覆盖远程", localTs, remoteTs)
}

// localUpdatedAtServer 本地 session 最后修改时间(以 id 最大值为近似, 已换算服务器时钟)
func (svc *Service) localUpdatedAtServer() int64 {
	raw, err := store.SessionsMaxUpdatedAt()
	if err != nil {
		log.Printf("[cloud] SessionsMaxUpdatedAt: %v", err)
		return 0
	}
	return svc.toServerTime(raw)
}

// applyRemotePayload 用远程 payload 覆盖本地(仅 session 文本)
func (svc *Service) applyRemotePayload(p store.CloudPayload) error {
	if svc.SessionSave == nil {
		return errors.New("未初始化 session 存储")
	}
	if p.Sessions == nil {
		p.Sessions = []store.CloudSession{}
	}
	return svc.SessionSave(p)
}

func (svc *Service) currentCloudPw() string {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.cloudPw
}

// ===========================================================================
// 云端配置操作
// ===========================================================================

// UploadConfig 上传配置到云端
// 参数: password 加密密码; reset=true 清历史后新密码全新上传; force=true 直接上传
func (svc *Service) UploadConfig(ctx context.Context, password string, reset, force bool) (map[string]interface{}, error) {
	if svc.Token() == "" {
		return map[string]interface{}{"ok": false, "error": "请先登录"}, nil
	}
	pw := password
	if pw == "" && svc.CloudPwValid() {
		pw = svc.currentCloudPw()
	}
	if pw == "" {
		return map[string]interface{}{"ok": false, "needPassword": true, "error": "请输入云端加密密码"}, nil
	}
	if len(pw) < 6 {
		return map[string]interface{}{"ok": false, "error": "加密密码至少 6 位"}, nil
	}
	plain, err := svc.buildCloudPayload()
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}, nil
	}
	plainStr := canonicalPayload(plain)

	if reset {
		_, _ = svc.apiRequest(ctx, EPConfigDel, map[string]interface{}{"key": CloudConfigKey}, true)
		blob, _ := EncryptData(plainStr, pw)
		blobJSON, _ := json.Marshal(blob)
		_, err := svc.apiRequest(ctx, EPConfigSave, map[string]interface{}{
			"key": CloudConfigKey, "data": string(blobJSON),
		}, true)
		if err != nil {
			return svc.uploadError(err)
		}
		svc.SaveCloudSyncPw(pw)
		return map[string]interface{}{"ok": true, "reset": true}, nil
	}

	cmp, err := svc.compareRemote(ctx, plainStr, pw)
	if err != nil {
		if strings.Contains(err.Error(), "解密失败") {
			return map[string]interface{}{
				"ok": false, "needPassword": true, "resetOption": true,
				"error": "加密密码无法解密历史版本数据(可能已在其他电脑更换密码)",
			}, nil
		}
		return svc.uploadError(err)
	}
	if cmp == "noData" {
		blob, _ := EncryptData(plainStr, pw)
		blobJSON, _ := json.Marshal(blob)
		_, err := svc.apiRequest(ctx, EPConfigSave, map[string]interface{}{
			"key": CloudConfigKey, "data": string(blobJSON),
		}, true)
		if err != nil {
			return svc.uploadError(err)
		}
		svc.SaveCloudSyncPw(pw)
		return map[string]interface{}{"ok": true, "reset": true}, nil
	}
	if cmp == "same" {
		svc.SaveCloudSyncPw(pw)
		return map[string]interface{}{"ok": true, "skipped": true}, nil
	}
	if !force {
		svc.SaveCloudSyncPw(pw)
		return map[string]interface{}{"ok": true, "conflict": true}, nil
	}
	blob, _ := EncryptData(plainStr, pw)
	blobJSON, _ := json.Marshal(blob)
	_, err = svc.apiRequest(ctx, EPConfigSave, map[string]interface{}{
		"key": CloudConfigKey, "data": string(blobJSON),
	}, true)
	if err != nil {
		return svc.uploadError(err)
	}
	svc.SaveCloudSyncPw(pw)
	return map[string]interface{}{"ok": true, "reset": false}, nil
}

// uploadError 统一处理上传错误
func (svc *Service) uploadError(err error) (map[string]interface{}, error) {
	if strings.Contains(err.Error(), "其他设备") {
		svc.notifyTokenInvalid()
		return map[string]interface{}{"ok": false, "error": "该账号已在其他设备登录, 本地登录已失效, 请重新登录"}, nil
	}
	if strings.Contains(err.Error(), "解密") {
		return map[string]interface{}{
			"ok": false, "needPassword": true, "resetOption": true,
			"error": "加密密码无法解密历史版本数据, 可重新输入密码或使用新密码并清除历史",
		}, nil
	}
	return map[string]interface{}{"ok": false, "error": err.Error()}, nil
}

// ListVersions 获取云端配置版本列表
func (svc *Service) ListVersions(ctx context.Context) (map[string]interface{}, error) {
	if svc.Token() == "" {
		return map[string]interface{}{"ok": false, "error": "请先登录"}, nil
	}
	res, err := svc.apiRequest(ctx, EPConfigList, map[string]interface{}{"key": CloudConfigKey}, true)
	if err != nil {
		if strings.Contains(err.Error(), "其他设备") {
			svc.notifyTokenInvalid()
			return map[string]interface{}{"ok": false, "error": "该账号已在其他设备登录, 本地登录已失效, 请重新登录"}, nil
		}
		return map[string]interface{}{"ok": false, "error": err.Error()}, nil
	}
	versions, _ := res.([]interface{})
	if versions == nil {
		versions = []interface{}{}
	}
	return map[string]interface{}{"ok": true, "versions": versions}, nil
}

// AutoSyncIgnore 自动同步-解密失败选择 [A] 忽略: 清本地密码 + 停止自动同步
func (svc *Service) AutoSyncIgnore() map[string]interface{} {
	svc.StopAutoSync()
	svc.ClearCloudSyncPw()
	return map[string]interface{}{"ok": true}
}

// AutoSyncDismiss 自动同步-冲突弹窗"暂不处理"
func (svc *Service) AutoSyncDismiss() map[string]interface{} {
	return map[string]interface{}{"ok": true}
}

// AutoSyncRetryWithPassword 自动同步-解密失败选择 [B] 用新密码重试
func (svc *Service) AutoSyncRetryWithPassword(ctx context.Context, password string) (map[string]interface{}, error) {
	pw := password
	if pw == "" {
		return map[string]interface{}{"ok": false, "undecryptable": true, "error": "请输入加密密码"}, nil
	}
	if len(pw) < 6 {
		return map[string]interface{}{"ok": false, "error": "加密密码至少 6 位"}, nil
	}
	res, err := svc.apiRequest(ctx, EPConfigGet, map[string]interface{}{"key": CloudConfigKey}, true)
	if err != nil {
		if strings.Contains(err.Error(), "其他设备") {
			svc.notifyTokenInvalid()
			return map[string]interface{}{"ok": false, "error": "该账号已在其他设备登录, 本地登录已失效, 请重新登录"}, nil
		}
		return map[string]interface{}{"ok": false, "error": err.Error()}, nil
	}
	m, _ := res.(map[string]interface{})
	if m == nil || m["data"] == nil {
		svc.ClearSyncPending()
		return map[string]interface{}{"ok": true, "status": "noData"}, nil
	}
	var blob EncryptedBlob
	dataStr, _ := m["data"].(string)
	if err := json.Unmarshal([]byte(dataStr), &blob); err != nil {
		return map[string]interface{}{"ok": false, "undecryptable": true, "error": "该密码仍无法解密云端数据, 可继续尝试或选择其他方案"}, nil
	}
	remotePlain, err := DecryptData(&blob, pw)
	if err != nil {
		return map[string]interface{}{"ok": false, "undecryptable": true, "error": "该密码仍无法解密云端数据, 可继续尝试或选择其他方案"}, nil
	}
	svc.SaveCloudSyncPw(pw)
	local, _ := svc.buildCloudPayload()
	status := "diff"
	if canonicalPayload(mustPayload(remotePlain)) == canonicalPayload(local) {
		status = "same"
	}
	return map[string]interface{}{"ok": true, "status": status}, nil
}

// DownloadConfig 从云端下载配置: 拉取指定版本(version<=0 取最新), 解密后【不落盘】返回
func (svc *Service) DownloadConfig(ctx context.Context, version int, password string) (map[string]interface{}, error) {
	if svc.Token() == "" {
		return map[string]interface{}{"ok": false, "error": "请先登录"}, nil
	}
	pw := password
	if pw == "" && svc.CloudPwValid() {
		pw = svc.currentCloudPw()
	}
	if pw == "" {
		return map[string]interface{}{"ok": false, "needPassword": true, "error": "请输入云端加密密码"}, nil
	}
	res, err := svc.apiRequest(ctx, EPConfigGet, map[string]interface{}{
		"key": CloudConfigKey, "version": version,
	}, true)
	if err != nil {
		if strings.Contains(err.Error(), "其他设备") {
			svc.notifyTokenInvalid()
			return map[string]interface{}{"ok": false, "error": "该账号已在其他设备登录, 本地登录已失效, 请重新登录"}, nil
		}
		return map[string]interface{}{"ok": false, "error": err.Error()}, nil
	}
	m, _ := res.(map[string]interface{})
	if m == nil || m["data"] == nil {
		return map[string]interface{}{"ok": false, "error": "云端没有已上传的配置"}, nil
	}
	var p store.CloudPayload
	{
		var blob EncryptedBlob
		dataStr, _ := m["data"].(string)
		if err := json.Unmarshal([]byte(dataStr), &blob); err != nil {
			return undecryptableResult(password), nil
		}
		plain, err := DecryptData(&blob, pw)
		if err != nil {
			return undecryptableResult(password), nil
		}
		if err := json.Unmarshal([]byte(plain), &p); err != nil {
			return undecryptableResult(password), nil
		}
	}
	if p.Sessions == nil {
		p.Sessions = []store.CloudSession{}
	}
	if p.QuickCommands == nil {
		p.QuickCommands = []store.CloudQuickCommand{}
	}
	// 与本地最新数据对比
	same := false
	{
		local, _ := svc.buildCloudPayload()
		same = canonicalPayload(local) == canonicalPayload(p)
	}
	svc.SaveCloudSyncPw(pw)
	return map[string]interface{}{
		"ok": true,
		"count": len(p.Sessions), "sessions": p.Sessions,
		"qcCount": len(p.QuickCommands), "quickCommands": p.QuickCommands,
		"same": same,
	}, nil
}

// undecryptableResult 解密失败返回
func undecryptableResult(password string) map[string]interface{} {
	return map[string]interface{}{
		"ok": false, "needPassword": true, "undecryptable": true, "resetOption": true,
		"error": "加密密码错误, 无法解密云端配置",
	}
}

// ApplyDownloaded 应用已下载的云端配置(前端决策"用远程覆盖本地"后调用)
func (svc *Service) ApplyDownloaded(payload store.CloudPayload) (map[string]interface{}, error) {
	if svc.SessionSave == nil {
		return map[string]interface{}{"ok": false, "error": "未初始化 session 存储"}, nil
	}
	if payload.Sessions == nil {
		payload.Sessions = []store.CloudSession{}
	}
	if payload.QuickCommands == nil {
		payload.QuickCommands = []store.CloudQuickCommand{}
	}
	if err := svc.SessionSave(payload); err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}, nil
	}
	return map[string]interface{}{
		"ok": true, "count": len(payload.Sessions), "qcCount": len(payload.QuickCommands),
	}, nil
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}
