package cloud

import (
	_ "embed"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// Config 云端后端地址 + 各接口路径配置
//
// 设计: 把基地址与所有接口路径集中到 JSON 配置, 而不是硬编码在 Service 的各处 —
// 避免出现"改一处忘改一处"的散落隐患, 也方便在不同环境(开发/测试/生产)切换。
//
// 加载优先级(对齐 LdTools):
//   1. exe 同目录 data/cloud-config.json(用户可编辑, 最高优先)
//   2. 内嵌默认(go:embed assets/cloud-config.json, 保证单文件便携版开箱即用)
//   3. 兜底常量 DefaultConfig()(嵌入失败时使用)
type Config struct {
	BaseURL   string            `json:"baseUrl"`
	Endpoints map[string]string `json:"endpoints"`
}

// ConfigFileName 配置文件名(置于 data 目录, 与 cloud_state.json 同级)
const ConfigFileName = "cloud-config.json"

//go:embed assets/cloud-config.json
var embeddedConfig []byte

// endpoint 命名常量 — Service 内部按名字而非裸路径调用, 集中注册避免散落
const (
	EPVcode      = "vcode"
	EPLogin      = "login"
	EPRegister   = "register"
	EPConfigGet  = "configGet"
	EPConfigSave = "configSave"
	EPConfigDel  = "configDel"
	EPConfigList = "configList"
)

// loadEmbedded 解析嵌入的默认配置(启动时一次性)
var embeddedParsed *Config

func loadEmbedded() *Config {
	if embeddedParsed != nil {
		return embeddedParsed
	}
	cfg, err := parseConfigJSON(embeddedConfig)
	if err != nil {
		log.Printf("[cloud] 嵌入默认配置解析失败: %v (fallback to DefaultConfig)", err)
		return DefaultConfig()
	}
	embeddedParsed = cfg
	return cfg
}

// parseConfigJSON 解析 JSON 为 Config; 失败返回 DefaultConfig() 与错误
func parseConfigJSON(raw []byte) (*Config, error) {
	var file struct {
		BaseURL   string            `json:"baseUrl"`
		Endpoints map[string]string `json:"endpoints"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	if file.BaseURL != "" {
		cfg.BaseURL = file.BaseURL
	}
	for k, v := range file.Endpoints {
		if v != "" {
			cfg.Endpoints[k] = v
		}
	}
	return cfg, nil
}

// DefaultConfig 返回内置兜底默认配置; 云端接口与 LdTools 共用同一套 LdMain 后端,
// 默认地址一致: https://api.gxlidang.com (生产)
func DefaultConfig() *Config {
	return &Config{
		BaseURL: "https://api.gxlidang.com",
		Endpoints: map[string]string{
			EPVcode:      "/get/vcode",
			EPLogin:      "/auto/login",
			EPRegister:   "/auto/reg",
			EPConfigGet:  "/user/config/get",
			EPConfigSave: "/user/config/save",
			EPConfigDel:  "/user/config/del",
			EPConfigList: "/user/config/list",
		},
	}
}

// LoadConfig 从 dataDir/cloud-config.json 加载配置(优先级 1, 缺失字段用嵌入默认补齐)。
// 文件不存在时自动写入一份(便于用户找到并修改)。
// 加载优先级: 用户文件 → 嵌入默认 → 兜底常量。
func LoadConfig(dataDir string) (*Config, error) {
	path := filepath.Join(dataDir, ConfigFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		base := loadEmbedded()
		_ = base.Save(dataDir)
		return base, nil
	}
	embedded := loadEmbedded()
	userCfg, jerr := parseConfigJSON(raw)
	if jerr != nil {
		log.Printf("[cloud] %s 解析失败: %v (fallback to embedded)", path, jerr)
		_ = embedded.Save(dataDir)
		return embedded, nil
	}
	for k, v := range userCfg.Endpoints {
		embedded.Endpoints[k] = v
	}
	if userCfg.BaseURL != "" {
		embedded.BaseURL = userCfg.BaseURL
	}
	return embedded, nil
}

// Save 把当前配置写回 dataDir/cloud-config.json
func (c *Config) Save(dataDir string) error {
	if c == nil {
		return nil
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, ConfigFileName), b, 0o600)
}

// Endpoint 返回指定名字的接口路径(未注册则返回空字符串, 调用方应保证名字合法)
func (c *Config) Endpoint(name string) string {
	if c == nil {
		return ""
	}
	if p, ok := c.Endpoints[name]; ok {
		return p
	}
	return ""
}
