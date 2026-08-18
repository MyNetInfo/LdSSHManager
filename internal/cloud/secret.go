package cloud

import (
	"crypto/rand"
	"encoding/base64"
	"runtime"
	"strings"

	"LdSSHManager/internal/crypto"
	"github.com/zalando/go-keyring"
)

// 敏感字段落盘加密: 云端加密密码与登录 token 加密后写入 data/cloud_state.json,
// 避免明文落盘。加密失败时不落明文(置空, 由用户重新输入)。
//
// 平台策略:
//   - Windows: 用 DPAPI(当前用户)加密, 前缀 "dpapi:" (历史方案, 兼容旧数据);
//   - macOS/Linux: 首次生成随机 32 字节主密钥存入系统密钥环
//     (macOS Keychain / Linux Secret Service), 用该主密钥做 AES-256-GCM,
//     前缀 "kr:"。
//
// 旧版明文文件自动兼容: 无 "dpapi:"/"kr:" 前缀的值视为明文, 读取后触发一次加密重写迁移。

const (
	dpapiPrefix   = "dpapi:" // Windows DPAPI 密文
	keyringPrefix = "kr:"    // macOS/Linux 密钥环主密钥 + AES-GCM 密文
)

// 系统密钥环条目: service 固定应用名, user 固定为主密钥标识
const (
	keyringService = "LdSSHManager"
	keyringUser    = "cloud_secret_key" // 32 字节主密钥(base64)
)

// keyringMasterKey 从系统密钥环读取 32 字节主密钥; 不存在则随机生成并写入。
// 密钥环不可用时(如无桌面环境的 Linux)返回 error, 调用方按"不落明文"策略处理。
func keyringMasterKey() ([]byte, error) {
	s, err := keyring.Get(keyringService, keyringUser)
	if err == nil {
		return base64.StdEncoding.DecodeString(s)
	}
	if err != keyring.ErrNotFound {
		return nil, err
	}
	// 首次使用: 生成随机主密钥并写入密钥环
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	if err := keyring.Set(keyringService, keyringUser, base64.StdEncoding.EncodeToString(k)); err != nil {
		return nil, err
	}
	return k, nil
}

// encryptSecret 加密敏感字段为可落盘字符串; 空串原样返回空
func encryptSecret(plain string) string {
	if plain == "" {
		return ""
	}
	if runtime.GOOS == "windows" {
		c, err := crypto.DPAPIEncrypt([]byte(plain))
		if err != nil {
			return "" // 加密失败: 不落明文, 字段视为丢失
		}
		return dpapiPrefix + base64.StdEncoding.EncodeToString(c)
	}
	key, err := keyringMasterKey()
	if err != nil {
		return "" // 密钥环不可用: 不落明文, 字段视为丢失
	}
	s, err := crypto.SealWithKey([]byte(plain), key)
	if err != nil {
		return ""
	}
	return keyringPrefix + s
}

// decryptSecret 解密落盘的敏感字段; 兼容旧版明文(无前缀)直接原样返回
func decryptSecret(stored string) string {
	if stored == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(stored, dpapiPrefix):
		raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, dpapiPrefix))
		if err != nil {
			return ""
		}
		pt, err := crypto.DPAPIDecrypt(raw)
		if err != nil {
			return "" // 非本用户/机器或已损坏: 无法解密, 视为未登录
		}
		return string(pt)
	case strings.HasPrefix(stored, keyringPrefix):
		key, err := keyringMasterKey()
		if err != nil {
			return ""
		}
		pt, err := crypto.OpenWithKey(strings.TrimPrefix(stored, keyringPrefix), key)
		if err != nil {
			return "" // 主密钥已变(密钥环被清)或密文损坏: 视为未登录
		}
		return string(pt)
	default:
		return stored // 旧版明文
	}
}

// isEncryptedSecret 判断落盘值是否为密文格式(用于旧版明文迁移检测)
func isEncryptedSecret(stored string) bool {
	return strings.HasPrefix(stored, dpapiPrefix) || strings.HasPrefix(stored, keyringPrefix)
}
