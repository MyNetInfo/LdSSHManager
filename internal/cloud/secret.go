package cloud

import (
	"encoding/base64"
	"strings"

	"LdSSHManager/internal/crypto"
)

// 敏感字段落盘加密: 云端加密密码与登录 token 用 Windows DPAPI(当前用户)加密,
// 避免明文写在 data/cloud_state.json。加密失败时不落明文(置空, 由用户重新输入)。
// 旧版明文文件自动兼容: 无 "dpapi:" 前缀的值视为明文, 读取后触发一次加密重写迁移。

const secretPrefix = "dpapi:"

// encryptSecret 加密敏感字段为可落盘字符串; 空串原样返回空
func encryptSecret(plain string) string {
	if plain == "" {
		return ""
	}
	c, err := crypto.DPAPIEncrypt([]byte(plain))
	if err != nil {
		return "" // 加密失败: 不落明文, 字段视为丢失
	}
	return secretPrefix + base64.StdEncoding.EncodeToString(c)
}

// decryptSecret 解密落盘的敏感字段; 兼容旧版明文(无前缀)直接原样返回
func decryptSecret(stored string) string {
	if stored == "" {
		return ""
	}
	if !strings.HasPrefix(stored, secretPrefix) {
		return stored // 旧版明文
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, secretPrefix))
	if err != nil {
		return ""
	}
	pt, err := crypto.DPAPIDecrypt(raw)
	if err != nil {
		return "" // 非本用户/机器或已损坏: 无法解密, 视为未登录
	}
	return string(pt)
}

// isEncryptedSecret 判断落盘值是否为 DPAPI 密文格式(用于旧版明文迁移检测)
func isEncryptedSecret(stored string) bool {
	return strings.HasPrefix(stored, secretPrefix)
}
