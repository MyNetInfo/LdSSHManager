package store

import (
	"database/sql"
	"fmt"
)

// ensureKnownHostsTable 建立 known_hosts 表(每次 Open 时调用, IF NOT EXISTS 幂等)。
// 存储用户信任过的 SSH 服务端主机密钥, 用于首次连接确认/后续连接校验,
// 避免接受任意主机密钥(中间人攻击)。
func ensureKnownHostsTable(d *sql.DB) error {
	_, err := d.Exec(`
CREATE TABLE IF NOT EXISTS known_hosts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    host TEXT NOT NULL,
    key_type TEXT NOT NULL,
    key_blob TEXT NOT NULL,
    UNIQUE(host, key_type)
)`)
	if err != nil {
		return fmt.Errorf("create known_hosts: %w", err)
	}
	return nil
}

// GetKnownHost 返回已信任主机密钥的 blob(base64 marshalled ssh.PublicKey)。
// 未找到返回 ("", nil)。
func GetKnownHost(host, keyType string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database not open")
	}
	var blob string
	err := db.QueryRow(`SELECT key_blob FROM known_hosts WHERE host = ? AND key_type = ?`, host, keyType).Scan(&blob)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return blob, nil
}

// AddKnownHost 信任一个主机密钥: 未知主机确认后写入; 已存在则更新 blob(密钥轮换)。
// 使用先查后写, 避免依赖 UPSERT(go-sqlcipher 的 SQLite 版本较老不支持 ON CONFLICT)。
func AddKnownHost(host, keyType, blob string) error {
	if db == nil {
		return fmt.Errorf("database not open")
	}
	var existing string
	err := db.QueryRow(`SELECT key_blob FROM known_hosts WHERE host = ? AND key_type = ?`, host, keyType).Scan(&existing)
	switch {
	case err == sql.ErrNoRows:
		_, err = db.Exec(`INSERT INTO known_hosts (host, key_type, key_blob) VALUES (?, ?, ?)`,
			host, keyType, blob)
	case err == nil:
		_, err = db.Exec(`UPDATE known_hosts SET key_blob = ? WHERE host = ? AND key_type = ?`,
			blob, host, keyType)
	default:
		return err
	}
	if err != nil {
		return fmt.Errorf("add known host: %w", err)
	}
	return nil
}
