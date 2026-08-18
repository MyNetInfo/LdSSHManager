package store

// 云端同步用的数据结构(与 internal/cloud 包共享, 放在 store 避免循环依赖)。
// 注意: 同步【只含文本】—— 只同步 session 元数据(name/host/port/...) 与 快捷命令文本,
// 不包含 key 文件内容(二进制; key 文件只存路径, 本来就不入库)。

// CloudSession 云端 session(仅文本字段)
type CloudSession struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Username      string `json:"username"`
	AuthType      string `json:"authType"`
	KeyPath       string `json:"keyPath"`
	GroupName     string `json:"groupName"`
	SortOrder     int    `json:"sortOrder"`
	ProxyType     string `json:"proxyType"`
	ProxyHost     string `json:"proxyHost"`
	ProxyPort     int    `json:"proxyPort"`
	ProxyUsername string `json:"proxyUsername"`
	ExecCmd       string `json:"execCmd"`
	// 密码为文本字段, 用户已拍板随库保存; 同步时一并加密上传(云端 AES-256-GCM)。
	Password string `json:"password"`
}

// CloudQuickCommand 云端快捷命令(仅文本字段)
type CloudQuickCommand struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Command   string `json:"command"`
	WithEnter bool   `json:"withEnter"`
	SortOrder int    `json:"sortOrder"`
	Category  string `json:"category"`
}

// CloudPayload 上传到云端的配置(仅文本): 会话 + 快捷命令
type CloudPayload struct {
	Schema        int                `json:"schema"`
	Sessions      []CloudSession     `json:"sessions"`
	QuickCommands []CloudQuickCommand `json:"quickCommands"`
}

// LoadCloudPayload 读取本地全部 session + 快捷命令, 返回云端 payload(仅文本)。
func LoadCloudPayload() (CloudPayload, error) {
	list, err := ListSessions()
	if err != nil {
		return CloudPayload{}, err
	}
	qcs, err := ListQuickCommands()
	if err != nil {
		return CloudPayload{}, err
	}
	p := CloudPayload{Schema: 2, Sessions: []CloudSession{}, QuickCommands: []CloudQuickCommand{}}
	for _, s := range list {
		p.Sessions = append(p.Sessions, CloudSession{
			ID:            s.ID,
			Name:          s.Name,
			Host:          s.Host,
			Port:          s.Port,
			Username:      s.Username,
			AuthType:      s.AuthType,
			KeyPath:       s.KeyPath,
			GroupName:     s.GroupName,
			SortOrder:     s.SortOrder,
			ProxyType:     s.ProxyType,
			ProxyHost:     s.ProxyHost,
			ProxyPort:     s.ProxyPort,
			ProxyUsername: s.ProxyUsername,
			ExecCmd:       s.ExecCmd,
			Password:      s.Password,
		})
	}
	for _, q := range qcs {
		p.QuickCommands = append(p.QuickCommands, CloudQuickCommand{
			ID:        q.ID,
			Name:      q.Name,
			Command:   q.Command,
			WithEnter: q.WithEnter,
			SortOrder: q.SortOrder,
			Category:  q.Category,
		})
	}
	return p, nil
}

// ReplaceAllFromCloud 用云端下载的数据全量覆盖本地(仅文本): 会话 + 快捷命令。
func ReplaceAllFromCloud(p CloudPayload) error {
	list := make([]Session, 0, len(p.Sessions))
	for _, c := range p.Sessions {
		list = append(list, Session{
			ID:            c.ID,
			Name:          c.Name,
			Host:          c.Host,
			Port:          c.Port,
			Username:      c.Username,
			AuthType:      c.AuthType,
			KeyPath:       c.KeyPath,
			GroupName:     c.GroupName,
			SortOrder:     c.SortOrder,
			ProxyType:     c.ProxyType,
			ProxyHost:     c.ProxyHost,
			ProxyPort:     c.ProxyPort,
			ProxyUsername: c.ProxyUsername,
			ExecCmd:       c.ExecCmd,
			Password:      c.Password,
		})
	}
	if err := ReplaceAllSessions(list); err != nil {
		return err
	}
	qcs := make([]QuickCommand, 0, len(p.QuickCommands))
	for _, c := range p.QuickCommands {
		qcs = append(qcs, QuickCommand{
			ID:        c.ID,
			Name:      c.Name,
			Command:   c.Command,
			WithEnter: c.WithEnter,
			SortOrder: c.SortOrder,
			Category:  c.Category,
		})
	}
	return ReplaceAllQuickCommands(qcs)
}
