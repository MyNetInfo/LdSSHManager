// Package crypto 提供"密码 + 随机 salt"派生的对称加密工具, 用于本地备份文件的加密。
//
// 设计目标: 备份文件用"备份时的锁屏密码"(未设锁屏密码则用默认 123456)加密,
// 由于 salt 随文件随机生成, 同一密码+明文每次加密结果不同; 密钥派生参数与
// vault.KeyFromPassword 保持一致(scrypt N=16384, r=8, p=1, keyLen=32),
// 但本包只派生原始密钥字节用于 AES-GCM, 不直接参与 SQLCipher 开库。
//
// 输出为可直接落盘的 JSON 文本信封, 与 "本机 vault.lock 中的 hash" 解耦,
// 因此换一台电脑也能用"备份时的密码"解密, 不依赖原机器的锁屏校验物。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"golang.org/x/crypto/scrypt"
)

const (
	scryptN = 16384
	scryptR = 8
	scryptP = 1
	keyLen  = 32
	saltLen = 16
)

// envelope 加密信封: salt/nonce 随机, ct 为 AES-GCM 密文(base64)。
type envelope struct {
	V     int    `json:"v"`
	App   string `json:"app"`
	KDF   string `json:"kdf"`
	Salt  string `json:"salt"`
	Nonce string `json:"nonce"`
	CT    string `json:"ct"`
}

// deriveKey 由密码 + salt 派生 32 字节原始密钥(scrypt, 与 vault.KeyFromPassword 同参)。
func deriveKey(password string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(password), salt, scryptN, scryptR, scryptP, keyLen)
}

// EncryptString 用密码对明文做 AES-256-GCM 加密, 返回可存盘的 JSON 文本信封。
// salt/nonce 每次随机 —— 即使相同密码 + 相同明文, 每次输出的密文都不同。
func EncryptString(plaintext, password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", err
	}
	key, err := deriveKey(password, salt)
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
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	env := envelope{
		V:     1,
		App:   "LdSSHManager",
		KDF:   "scrypt",
		Salt:  hex.EncodeToString(salt),
		Nonce: hex.EncodeToString(nonce),
		CT:    base64.StdEncoding.EncodeToString(ct),
	}
	b, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecryptString 解密 EncryptString 产生的信封; 密码错误或数据被篡改都会返回 error
// (AES-GCM 自带完整性校验, 不会吐出半截脏数据)。
func DecryptString(envelopeStr, password string) (string, error) {
	var env envelope
	if err := json.Unmarshal([]byte(envelopeStr), &env); err != nil {
		return "", errors.New("备份文件格式无效")
	}
	if env.App != "LdSSHManager" || env.V != 1 || env.KDF != "scrypt" {
		return "", errors.New("不是有效的 LdSSHManager 备份")
	}
	salt, err := hex.DecodeString(env.Salt)
	if err != nil {
		return "", errors.New("备份文件格式无效")
	}
	nonce, err := hex.DecodeString(env.Nonce)
	if err != nil {
		return "", errors.New("备份文件格式无效")
	}
	ct, err := base64.StdEncoding.DecodeString(env.CT)
	if err != nil {
		return "", errors.New("备份文件格式无效")
	}
	key, err := deriveKey(password, salt)
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
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", errors.New("密码错误, 无法解密备份 (或文件已损坏)")
	}
	return string(plain), nil
}
