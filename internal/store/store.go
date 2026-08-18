package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"LdSSHManager/internal/config"
	_ "github.com/mutecomm/go-sqlcipher"
	_ "modernc.org/sqlite"
)

// Session represents a saved SSH connection profile.
type Session struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	// Password and private key content are NEVER stored. Only the auth type
	// and (optionally) a path to the key file on disk are persisted; the actual
	// secret is supplied by the user at connect time.
	AuthType  string `json:"authType"` // "password" | "key"
	KeyPath   string `json:"keyPath"`  // absolute path to key file on disk (not the key itself)
	GroupName string `json:"groupName"`
	SortOrder int    `json:"sortOrder"`
	// Proxy settings. Proxy password is not persisted; it is supplied at connect time.
	ProxyType     string `json:"proxyType"` // "" | "none" | "socks5" | "http"
	ProxyHost     string `json:"proxyHost"`
	ProxyPort     int    `json:"proxyPort"`
	ProxyUsername string `json:"proxyUsername"`
	// ExecCmd is run in the remote shell right after connect (e.g. "ls -la").
	ExecCmd string `json:"execCmd"`
	// Password is stored per the user's explicit choice (2026-08-12) so that
	// sessions can auto-connect without prompting. Stored encrypted at rest
	// via SQLCipher when a lock-screen password is set (see internal/vault).
	Password string `json:"password"`
}

// QuickCommand 是底部"快捷命令"栏的按钮: 点击向当前终端发送 Command(可选附加回车)。
type QuickCommand struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`      // 按钮上显示的文字
	Command   string `json:"command"`   // 点击时发送到终端的命令
	WithEnter bool   `json:"withEnter"` // 发送后是否附加回车(自动执行)
	SortOrder int    `json:"sortOrder"` // 排序(升序)
	Category  string `json:"category"`  // 分类(用户自定义自由文本, 空串=未分类)
}

var db *sql.DB

// sqliteHeader 明文 SQLite 文件头(用于检测旧版明文库, 触发自动迁移)
var sqliteHeader = []byte("SQLite format 3\x00")

// isPlainSQLite 判断 dbPath 是否为明文 SQLite 数据库(旧版 modernc 创建)。
// SQLCipher 加密库的文件头是随机密文, 不会是 "SQLite format 3\0"。
func isPlainSQLite(dbPath string) bool {
	f, err := os.Open(dbPath)
	if err != nil {
		return false
	}
	defer f.Close()
	var head [16]byte
	n, _ := f.Read(head[:])
	if n != 16 {
		return false
	}
	return string(head[:]) == string(sqliteHeader)
}

// Open 用指定 SQLCipher key(hex) 打开数据库并建表。
// keyHex 为空时按明文打开(仅用于内部迁移读取)。
func Open(keyHex string) error {
	dbPath := config.DBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}

	// 旧版明文库 → 迁移为 SQLCipher 加密库(仅首次升级时发生一次)
	if isPlainSQLite(dbPath) {
		if err := migratePlainToEncrypted(dbPath, keyHex); err != nil {
			return fmt.Errorf("migrate plain db: %w", err)
		}
	}

	dsn := dbPath
	if keyHex != "" {
		dsn = dbPath + "?_pragma_key=x'" + keyHex + "'&_foreign_keys=1"
	}
	var err error
	db, err = sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	return migrate()
}

// IsOpen 数据库是否已打开(未启用锁屏密码时打开; 已启用时解锁后打开)。
func IsOpen() bool {
	return db != nil
}

// Rekey 把数据库 key 切换到新 key(设置/修改/取消锁屏密码时调用)。
// 执行后关闭连接池并用新 key 重新打开, 确保所有连接持有新 key。
// 注意: rekey 前先把连接池压到单连接(SetMaxOpenConns(1) + 取唯一连接),
// 避免连接池中旧 key 的连接在 rekey 期间读到错页/写回, 产生损坏行。
func Rekey(keyHex string) error {
	if db == nil {
		return errors.New("database not open")
	}
	ctx := context.Background()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "PRAGMA rekey = \"x'"+keyHex+"'\""); err != nil {
		return err
	}
	// 重开连接池(新 key); 保留旧句柄的错误不覆盖 rekey 结果
	old := db
	db = nil
	_ = old.Close()
	return Open(keyHex)
}

// migratePlainToEncrypted 把旧版明文库迁移为 SQLCipher 加密库:
//  1. 用 modernc(明文驱动) 读取全部数据
//  2. 原文件改名备份为 <db>.plain.bak
//  3. 用 SQLCipher + key 重建新库并写入数据
func migratePlainToEncrypted(dbPath, keyHex string) error {
	plain, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer plain.Close()

	// 读取全部数据(表结构由新库 migrate() 重建, 这里只搬数据)
	type kv struct{ key, value string }
	var settings []kv
	rows, err := plain.Query(`SELECT key, value FROM settings`)
	if err != nil {
		// settings 表可能不存在(旧库), 忽略
		if !strings.Contains(err.Error(), "no such table") {
			return err
		}
	} else {
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err != nil {
				rows.Close()
				return err
			}
			settings = append(settings, kv{k, v})
		}
		rows.Close()
	}

	type sessionRow struct {
		s Session
	}
	var sessions []sessionRow
	rows2, err := plain.Query(`SELECT id, name, host, port, username, auth_type, key_path, group_name, sort_order,
		proxy_type, proxy_host, proxy_port, proxy_username, exec_cmd, password FROM sessions`)
	if err != nil {
		if !strings.Contains(err.Error(), "no such table") {
			return err
		}
	} else {
		for rows2.Next() {
			var s Session
			if err := rows2.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Username, &s.AuthType, &s.KeyPath,
				&s.GroupName, &s.SortOrder, &s.ProxyType, &s.ProxyHost, &s.ProxyPort, &s.ProxyUsername,
				&s.ExecCmd, &s.Password); err != nil {
				rows2.Close()
				return err
			}
			sessions = append(sessions, sessionRow{s})
		}
		rows2.Close()
	}

	// 原文件改名备份(成功后新库会替换该路径)
	if err := os.Rename(dbPath, dbPath+".plain.bak"); err != nil {
		return err
	}

	// 用 SQLCipher 重建
	dsn := dbPath
	if keyHex != "" {
		dsn = dbPath + "?_pragma_key=x'" + keyHex + "'&_foreign_keys=1"
	}
	enc, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return err
	}
	defer enc.Close()
	if err := migrateWith(enc); err != nil {
		return err
	}
	for _, kv := range settings {
		if _, err := enc.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)`, kv.key, kv.value); err != nil {
			return err
		}
	}
	for _, r := range sessions {
		s := r.s
		if _, err := enc.Exec(`INSERT INTO sessions (id, name, host, port, username, auth_type, key_path,
			group_name, sort_order, proxy_type, proxy_host, proxy_port, proxy_username, exec_cmd, password)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.ID, s.Name, s.Host, s.Port, s.Username, s.AuthType, s.KeyPath, s.GroupName, s.SortOrder,
			s.ProxyType, s.ProxyHost, s.ProxyPort, s.ProxyUsername, s.ExecCmd, s.Password); err != nil {
			return err
		}
	}
	return nil
}

// Close releases the database handle.
// 注意: 关闭后必须把 db 置 nil —— 否则 IsOpen() 误报 true(db != nil),
// 锁屏后 openDBIfNeeded 会跳过重新开库, 后续 Rekey 在已关闭连接池上报
// "sql: database is closed"(修改/取消锁屏密码在锁屏态必现)。
func Close() error {
	if db == nil {
		return nil
	}
	err := db.Close()
	db = nil
	return err
}

// migrateWith 用指定连接执行建表(与 migrate 相同, 供迁移/正常路径共用)。
func migrateWith(d *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    host TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 22,
    username TEXT NOT NULL,
    auth_type TEXT NOT NULL DEFAULT 'password',
    key_path TEXT DEFAULT '',
    group_name TEXT NOT NULL DEFAULT 'default',
    sort_order INTEGER NOT NULL DEFAULT 0,
    proxy_type TEXT DEFAULT '',
    proxy_host TEXT DEFAULT '',
    proxy_port INTEGER DEFAULT 0,
    proxy_username TEXT DEFAULT ''
);
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT
);
CREATE TABLE IF NOT EXISTS quick_commands (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    command TEXT NOT NULL DEFAULT '',
    with_enter INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    category TEXT NOT NULL DEFAULT ''
);
`
	if _, err := d.Exec(schema); err != nil {
		return err
	}

	// Add proxy columns if they don't exist (migration from older schema).
	proxyCols := []string{
		"ALTER TABLE sessions ADD COLUMN proxy_type TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN proxy_host TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN proxy_port INTEGER DEFAULT 0",
		"ALTER TABLE sessions ADD COLUMN proxy_username TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN exec_cmd TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN password TEXT DEFAULT ''",
	}
	for _, col := range proxyCols {
		if _, err := d.Exec(col); err != nil {
			// Ignore "duplicate column name" errors.
			if !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
		}
	}

	// Add category column to quick_commands if it doesn't exist (migration from older schema).
	quickCols := []string{
		"ALTER TABLE quick_commands ADD COLUMN category TEXT NOT NULL DEFAULT ''",
	}
	for _, col := range quickCols {
		if _, err := d.Exec(col); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
		}
	}
	if err := ensureKnownHostsTable(d); err != nil {
		return err
	}
	return nil
}

func migrate() error {
	return migrateWith(db)
}

// ListSessions returns all saved sessions ordered by group and sort order.
func ListSessions() ([]Session, error) {
	rows, err := db.Query(`
		SELECT id, name, host, port, username, auth_type, key_path, group_name, sort_order,
		       proxy_type, proxy_host, proxy_port, proxy_username, exec_cmd, password
		FROM sessions
		ORDER BY group_name, sort_order, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list = make([]Session, 0)
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Username, &s.AuthType, &s.KeyPath, &s.GroupName, &s.SortOrder,
			&s.ProxyType, &s.ProxyHost, &s.ProxyPort, &s.ProxyUsername, &s.ExecCmd, &s.Password); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// GetSession loads a single session by id.
func GetSession(id int64) (*Session, error) {
	row := db.QueryRow(`
		SELECT id, name, host, port, username, auth_type, key_path, group_name, sort_order,
		       proxy_type, proxy_host, proxy_port, proxy_username, exec_cmd, password
		FROM sessions WHERE id = ?
	`, id)
	var s Session
	if err := row.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Username, &s.AuthType, &s.KeyPath, &s.GroupName, &s.SortOrder,
		&s.ProxyType, &s.ProxyHost, &s.ProxyPort, &s.ProxyUsername, &s.ExecCmd, &s.Password); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveSession inserts or updates a session.
func SaveSession(s *Session) error {
	if s.ID == 0 {
		res, err := db.Exec(`
			INSERT INTO sessions (name, host, port, username, auth_type, key_path, group_name, sort_order,
			                    proxy_type, proxy_host, proxy_port, proxy_username, exec_cmd, password)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, s.Name, s.Host, s.Port, s.Username, s.AuthType, s.KeyPath, s.GroupName, s.SortOrder,
			s.ProxyType, s.ProxyHost, s.ProxyPort, s.ProxyUsername, s.ExecCmd, s.Password)
		if err != nil {
			return err
		}
		s.ID, _ = res.LastInsertId()
		return nil
	}
	_, err := db.Exec(`
		UPDATE sessions
		SET name=?, host=?, port=?, username=?, auth_type=?, key_path=?, group_name=?, sort_order=?,
		    proxy_type=?, proxy_host=?, proxy_port=?, proxy_username=?, exec_cmd=?, password=?
		WHERE id=?
	`, s.Name, s.Host, s.Port, s.Username, s.AuthType, s.KeyPath, s.GroupName, s.SortOrder,
		s.ProxyType, s.ProxyHost, s.ProxyPort, s.ProxyUsername, s.ExecCmd, s.Password, s.ID)
	return err
}

// DeleteSession removes a session by id.
func DeleteSession(id int64) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DuplicateSession creates a copy of an existing session with a unique name.
func DuplicateSession(id int64) (*Session, error) {
	orig, err := GetSession(id)
	if err != nil {
		return nil, err
	}
	name, err := makeUniqueName(orig.Name)
	if err != nil {
		return nil, err
	}
	copy := &Session{
		Name:          name,
		Host:          orig.Host,
		Port:          orig.Port,
		Username:      orig.Username,
		AuthType:      orig.AuthType,
		KeyPath:       orig.KeyPath,
		GroupName:     orig.GroupName,
		SortOrder:     orig.SortOrder,
		ProxyType:     orig.ProxyType,
		ProxyHost:     orig.ProxyHost,
		ProxyPort:     orig.ProxyPort,
		ProxyUsername: orig.ProxyUsername,
		ExecCmd:       orig.ExecCmd,
		Password:      orig.Password,
	}
	if err := SaveSession(copy); err != nil {
		return nil, err
	}
	return copy, nil
}

func makeUniqueName(base string) (string, error) {
	candidate := base + " (copy)"
	exists, err := nameExists(candidate)
	if err != nil {
		return "", err
	}
	if !exists {
		return candidate, nil
	}
	for i := 1; i < 1000; i++ {
		candidate = fmt.Sprintf("%s (copy %d)", base, i)
		exists, err = nameExists(candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot generate unique name for %q", base)
}

func nameExists(name string) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE name = ?`, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetSetting returns a value from the settings table, or empty string if absent.
func GetSetting(key string) (string, error) {
	if db == nil {
		return "", errors.New("数据库未打开")
	}
	var value string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetSetting persists a key/value pair in the settings table.
// 使用先查后写, 避免依赖 UPSERT(go-sqlcipher 的 SQLite 版本较老不支持 ON CONFLICT)。
func SetSetting(key, value string) error {
	if db == nil {
		return errors.New("数据库未打开")
	}
	var existing string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&existing)
	switch {
	case err == sql.ErrNoRows:
		_, err = db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)`, key, value)
	case err == nil:
		_, err = db.Exec(`UPDATE settings SET value = ? WHERE key = ?`, value, key)
	default:
		return err
	}
	return err
}

// ReplaceAllSessions 用给定列表全量覆盖 sessions 表(导入/云端下载用)。
// 保留原有 id, 事务内先清空再插入。
func ReplaceAllSessions(list []Session) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM sessions`); err != nil {
		return err
	}
	for _, s := range list {
		var id int64
		if s.ID > 0 {
			id = s.ID
		}
		res, err := tx.Exec(`INSERT INTO sessions (id, name, host, port, username, auth_type, key_path,
			group_name, sort_order, proxy_type, proxy_host, proxy_port, proxy_username, exec_cmd, password)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, s.Name, s.Host, s.Port, s.Username, s.AuthType, s.KeyPath, s.GroupName, s.SortOrder,
			s.ProxyType, s.ProxyHost, s.ProxyPort, s.ProxyUsername, s.ExecCmd, s.Password)
		if err != nil {
			return err
		}
		if id == 0 {
			s.ID, _ = res.LastInsertId()
		}
	}
	return tx.Commit()
}

// SessionsMaxUpdatedAt 返回 session 表最大的 id(用于云端"谁最新用谁的"比较:
// LdSSHManager 无时间戳字段, 以 id 最大值近似最后修改序)。
func SessionsMaxUpdatedAt() (int64, error) {
	var maxID int64
	err := db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM sessions`).Scan(&maxID)
	if err != nil {
		return 0, err
	}
	return maxID, nil
}

// ===========================================================================
// 快捷命令 (quick_commands): 底部快捷命令栏
// ===========================================================================

// ListQuickCommands 返回全部快捷命令, 按 sort_order 升序。
// 单行 Scan 失败时跳过该行而不是整表报错: 某一条数据损坏时其余快捷命令仍可显示
// (否则一条坏行会让整个快捷命令栏空白, 只能手动删除该行才能恢复)。
func ListQuickCommands() ([]QuickCommand, error) {
	rows, err := db.Query(`SELECT id, name, command, with_enter, sort_order, category FROM quick_commands ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list = make([]QuickCommand, 0)
	for rows.Next() {
		var q QuickCommand
		var we int
		if err := rows.Scan(&q.ID, &q.Name, &q.Command, &we, &q.SortOrder, &q.Category); err != nil {
			log.Printf("ListQuickCommands 跳过损坏行: %v", err)
			continue
		}
		q.WithEnter = we != 0
		list = append(list, q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// SaveQuickCommand 新增或更新一条快捷命令。新增时 sort_order 取当前最大值 + 1。
func SaveQuickCommand(q *QuickCommand) error {
	if q.ID == 0 {
		var maxOrder int
		_ = db.QueryRow(`SELECT COALESCE(MAX(sort_order), 0) FROM quick_commands`).Scan(&maxOrder)
		res, err := db.Exec(
			`INSERT INTO quick_commands (name, command, with_enter, sort_order, category) VALUES (?, ?, ?, ?, ?)`,
			q.Name, q.Command, boolToInt(q.WithEnter), maxOrder+1, q.Category,
		)
		if err != nil {
			return err
		}
		q.ID, _ = res.LastInsertId()
		return nil
	}
	_, err := db.Exec(
		`UPDATE quick_commands SET name=?, command=?, with_enter=?, category=? WHERE id=?`,
		q.Name, q.Command, boolToInt(q.WithEnter), q.Category, q.ID,
	)
	return err
}

// DeleteQuickCommand 删除指定快捷命令。
func DeleteQuickCommand(id int64) error {
	_, err := db.Exec(`DELETE FROM quick_commands WHERE id = ?`, id)
	return err
}

// ReplaceAllQuickCommands 用给定列表全量覆盖 quick_commands 表(导入/云端下载用)。
// 保留原有 id, 事务内先清空再插入。
func ReplaceAllQuickCommands(list []QuickCommand) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM quick_commands`); err != nil {
		return err
	}
	for _, q := range list {
		var id int64
		if q.ID > 0 {
			id = q.ID
		}
		res, err := tx.Exec(
			`INSERT INTO quick_commands (id, name, command, with_enter, sort_order, category) VALUES (?, ?, ?, ?, ?, ?)`,
			id, q.Name, q.Command, boolToInt(q.WithEnter), q.SortOrder, q.Category,
		)
		if err != nil {
			return err
		}
		if id == 0 {
			q.ID, _ = res.LastInsertId()
		}
	}
	return tx.Commit()
}

// ReorderQuickCommands 按给定 id 顺序重排(每个 id 的 sort_order 设为列表索引)。
func ReorderQuickCommands(ids []int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE quick_commands SET sort_order=? WHERE id=?`, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
