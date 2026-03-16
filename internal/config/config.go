package config

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Category string

const (
	CategoryGeneral  Category = "general"
	CategoryDatabase Category = "database"
	CategoryWebUI    Category = "webui"
	CategoryLog      Category = "log"
	CategorySecurity Category = "security"
)

type ValueType string

const (
	ValueString ValueType = "string"
	ValueInt    ValueType = "int"
	ValueBool   ValueType = "bool"
)

type SettingDefinition struct {
	Name         string
	DBKey        string
	DefaultValue any
	Category     Category
	Type         ValueType
	Description  string
	Secret       bool
}

type PublicSetting struct {
	Name        string `json:"name"`
	DBKey       string `json:"db_key"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Secret      bool   `json:"secret"`
	Value       any    `json:"value"`
}

type PublicSection struct {
	Category string          `json:"category"`
	Title    string          `json:"title"`
	Items    []PublicSetting `json:"items"`
}

type Settings struct {
	AppName          string
	AppVersion       string
	Debug            bool
	DatabaseURL      string
	WebUIHost        string
	WebUIPort        string
	WebUISecretKey   string
	LogLevel         string
	LogFile          string
	LogRetentionDays int
	EncryptionKey    string
	portFromEnv      bool
}

var settingDefinitions = []SettingDefinition{
	{
		Name:         "app_name",
		DBKey:        "app.name",
		DefaultValue: "OpenAI/GPT 自动注册系统",
		Category:     CategoryGeneral,
		Type:         ValueString,
		Description:  "应用名称",
	},
	{
		Name:         "app_version",
		DBKey:        "app.version",
		DefaultValue: "2.0.0",
		Category:     CategoryGeneral,
		Type:         ValueString,
		Description:  "应用版本",
	},
	{
		Name:         "debug",
		DBKey:        "app.debug",
		DefaultValue: false,
		Category:     CategoryGeneral,
		Type:         ValueBool,
		Description:  "调试模式",
	},
	{
		Name:         "database_url",
		DBKey:        "database.url",
		DefaultValue: "data/database.db",
		Category:     CategoryDatabase,
		Type:         ValueString,
		Description:  "数据库路径或连接字符串",
	},
	{
		Name:         "webui_host",
		DBKey:        "webui.host",
		DefaultValue: "127.0.0.1",
		Category:     CategoryWebUI,
		Type:         ValueString,
		Description:  "Web UI 监听地址",
	},
	{
		Name:         "webui_port",
		DBKey:        "webui.port",
		DefaultValue: 8080,
		Category:     CategoryWebUI,
		Type:         ValueInt,
		Description:  "Web UI 监听端口",
	},
	{
		Name:         "webui_secret_key",
		DBKey:        "webui.secret_key",
		DefaultValue: "your-secret-key-change-in-production",
		Category:     CategoryWebUI,
		Type:         ValueString,
		Description:  "Web UI 密钥",
		Secret:       true,
	},
	{
		Name:         "log_level",
		DBKey:        "log.level",
		DefaultValue: "INFO",
		Category:     CategoryLog,
		Type:         ValueString,
		Description:  "日志级别",
	},
	{
		Name:         "log_file",
		DBKey:        "log.file",
		DefaultValue: "logs/app.log",
		Category:     CategoryLog,
		Type:         ValueString,
		Description:  "日志文件路径",
	},
	{
		Name:         "log_retention_days",
		DBKey:        "log.retention_days",
		DefaultValue: 30,
		Category:     CategoryLog,
		Type:         ValueInt,
		Description:  "日志保留天数",
	},
	{
		Name:         "encryption_key",
		DBKey:        "security.encryption_key",
		DefaultValue: "your-encryption-key-change-in-production",
		Category:     CategorySecurity,
		Type:         ValueString,
		Description:  "加密密钥",
		Secret:       true,
	},
}

func Load() Settings {
	loadDotEnv(".env")

	settings := Settings{
		AppName:          "OpenAI/GPT 自动注册系统",
		AppVersion:       "2.0.0",
		Debug:            false,
		DatabaseURL:      "data/database.db",
		WebUIHost:        "127.0.0.1",
		WebUIPort:        "8080",
		WebUISecretKey:   "your-secret-key-change-in-production",
		LogLevel:         "INFO",
		LogFile:          "logs/app.log",
		LogRetentionDays: 30,
		EncryptionKey:    "your-encryption-key-change-in-production",
	}

	if value := os.Getenv("APP_NAME"); value != "" {
		settings.AppName = value
	}
	if value := os.Getenv("APP_VERSION"); value != "" {
		settings.AppVersion = value
	}
	if value := os.Getenv("APP_DEBUG"); value != "" {
		settings.Debug = parseBool(value)
	}
	if value := os.Getenv("APP_DATABASE_URL"); value != "" {
		settings.DatabaseURL = value
	}
	if value := os.Getenv("APP_HOST"); value != "" {
		settings.WebUIHost = value
	}
	if value := os.Getenv("APP_PORT"); value != "" {
		settings.WebUIPort = value
		settings.portFromEnv = true
	}
	if value := os.Getenv("APP_WEBUI_SECRET_KEY"); value != "" {
		settings.WebUISecretKey = value
	}
	if value := os.Getenv("APP_LOG_LEVEL"); value != "" {
		settings.LogLevel = value
	}
	if value := os.Getenv("APP_LOG_FILE"); value != "" {
		settings.LogFile = value
	}
	if value := os.Getenv("APP_LOG_RETENTION_DAYS"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			settings.LogRetentionDays = parsed
		}
	}
	if value := os.Getenv("APP_ENCRYPTION_KEY"); value != "" {
		settings.EncryptionKey = value
	}

	return settings
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		_ = os.Setenv(key, value)
	}
}

func Definitions() []SettingDefinition {
	definitions := make([]SettingDefinition, len(settingDefinitions))
	copy(definitions, settingDefinitions)
	return definitions
}

func (s Settings) Addr() string {
	return net.JoinHostPort(s.WebUIHost, s.WebUIPort)
}

func (s Settings) PortProvided() bool {
	return s.portFromEnv
}

func (s Settings) DatabaseDriver() string {
	if strings.HasPrefix(s.DatabaseURL, "sqlite:///") {
		return "sqlite"
	}
	if strings.Contains(s.DatabaseURL, "://") {
		return "external"
	}
	return "sqlite"
}

func (s Settings) NormalizedDatabaseURL() string {
	if strings.HasPrefix(s.DatabaseURL, "sqlite:///") {
		return s.DatabaseURL
	}
	if strings.Contains(s.DatabaseURL, "://") {
		return s.DatabaseURL
	}
	if filepath.IsAbs(s.DatabaseURL) {
		return "sqlite:///" + filepath.ToSlash(s.DatabaseURL)
	}

	absPath, err := filepath.Abs(s.DatabaseURL)
	if err != nil {
		return "sqlite:///" + filepath.ToSlash(s.DatabaseURL)
	}
	return "sqlite:///" + filepath.ToSlash(absPath)
}

func (s Settings) SQLitePath() (string, bool) {
	if strings.HasPrefix(s.DatabaseURL, "sqlite:///") {
		path := filepath.FromSlash(strings.TrimPrefix(s.DatabaseURL, "sqlite:///"))
		if path == "" {
			return "", false
		}
		if filepath.IsAbs(path) {
			return filepath.Clean(path), true
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			return filepath.Clean(path), true
		}
		return filepath.Clean(absPath), true
	}

	if strings.Contains(s.DatabaseURL, "://") {
		return "", false
	}

	if filepath.IsAbs(s.DatabaseURL) {
		return filepath.Clean(s.DatabaseURL), true
	}

	absPath, err := filepath.Abs(s.DatabaseURL)
	if err != nil {
		return filepath.Clean(s.DatabaseURL), true
	}
	return filepath.Clean(absPath), true
}

func (s Settings) PublicSections() []PublicSection {
	sections := []PublicSection{
		{Category: string(CategoryGeneral), Title: "通用配置"},
		{Category: string(CategoryDatabase), Title: "数据库配置"},
		{Category: string(CategoryWebUI), Title: "Web UI 配置"},
		{Category: string(CategoryLog), Title: "日志配置"},
		{Category: string(CategorySecurity), Title: "安全配置"},
	}

	for _, definition := range settingDefinitions {
		item := PublicSetting{
			Name:        definition.Name,
			DBKey:       definition.DBKey,
			Type:        string(definition.Type),
			Description: definition.Description,
			Secret:      definition.Secret,
			Value:       s.valueFor(definition.Name, definition.Secret),
		}

		for index := range sections {
			if sections[index].Category == string(definition.Category) {
				sections[index].Items = append(sections[index].Items, item)
				break
			}
		}
	}

	return sections
}

func (s Settings) valueFor(name string, secret bool) any {
	var value any

	switch name {
	case "app_name":
		value = s.AppName
	case "app_version":
		value = s.AppVersion
	case "debug":
		value = s.Debug
	case "database_url":
		value = s.DatabaseURL
	case "webui_host":
		value = s.WebUIHost
	case "webui_port":
		value = s.WebUIPort
	case "webui_secret_key":
		value = s.WebUISecretKey
	case "log_level":
		value = s.LogLevel
	case "log_file":
		value = s.LogFile
	case "log_retention_days":
		value = s.LogRetentionDays
	case "encryption_key":
		value = s.EncryptionKey
	}

	if secret {
		return maskSecret(value)
	}
	return value
}

func parseBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func maskSecret(value any) string {
	text, ok := value.(string)
	if !ok || text == "" {
		return ""
	}
	if len(text) <= 4 {
		return "****"
	}
	return strings.Repeat("*", len(text)-4) + text[len(text)-4:]
}
