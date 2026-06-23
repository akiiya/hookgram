package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	DefaultDataDir = "data"
	DefaultPath    = "data/config.yaml"
	EnvConfigPath  = "HOOKGRAM_CONFIG"
	EnvDataDir     = "HOOKGRAM_DATA_DIR"
)

// Config 保存启动所需的基础配置，后续可热更新的基础项仍写回配置文件。
type Config struct {
	App      AppConfig      `yaml:"app"`
	Telegram TelegramConfig `yaml:"telegram"`
	Database DatabaseConfig `yaml:"database"`
	Security SecurityConfig `yaml:"security"`
	Log      LogConfig      `yaml:"log"`
	Path     string         `yaml:"-"`
}

type AppConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	BaseURL string `yaml:"base_url"`
}

type TelegramConfig struct {
	BotToken    string `yaml:"bot_token"`
	APIProxy    string `yaml:"api_proxy"`
	CommandMode string `yaml:"command_mode"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type SecurityConfig struct {
	SessionSecret   string `yaml:"session_secret"`
	TokenHashSecret string `yaml:"token_hash_secret"`
}

type LogConfig struct {
	Level string `yaml:"level"`
}

// Manager 以线程安全方式提供配置读取与写回。
type Manager struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

func LoadOrCreate(path string) (*Manager, error) {
	if path == "" {
		path = PathFromEnv()
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := Default()
		cfg.Path = path
		if err := writeTemplate(path, cfg); err != nil {
			return nil, err
		}
		return &Manager{path: path, cfg: cfg}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := Default()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if normalize(&cfg) {
		if err := writeTemplate(path, cfg); err != nil {
			return nil, err
		}
	}
	cfg.Path = path
	return &Manager{path: path, cfg: cfg}, nil
}

func Default() Config {
	return Config{
		App: AppConfig{
			Host: "127.0.0.1",
			Port: 8787,
		},
		Telegram: TelegramConfig{
			CommandMode: "polling",
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    defaultDatabaseDSN(),
		},
		Security: SecurityConfig{
			SessionSecret:   randomHex(32),
			TokenHashSecret: randomHex(32),
		},
		Log: LogConfig{Level: "info"},
	}
}

func PathFromEnv() string {
	if path := strings.TrimSpace(os.Getenv(EnvConfigPath)); path != "" {
		return path
	}
	return filepath.Join(dataDirFromEnv(), "config.yaml")
}

func (m *Manager) Current() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *Manager) Path() string {
	return m.path
}

func (m *Manager) Update(fn func(*Config)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.cfg
	fn(&next)
	normalize(&next)
	if err := writeTemplate(m.path, next); err != nil {
		return err
	}
	m.cfg = next
	return nil
}

func (a AppConfig) ListenAddr() string {
	host := strings.TrimSpace(a.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	port := a.Port
	if port == 0 {
		port = 8787
	}
	return fmt.Sprintf("%s:%d", host, port)
}

func (c Config) BaseURL() string {
	base := strings.TrimSpace(c.App.BaseURL)
	if base != "" {
		return strings.TrimRight(base, "/")
	}
	port := c.App.Port
	if port == 0 {
		port = 8787
	}
	return fmt.Sprintf("http://localhost:%d", port)
}

func (c Config) TelegramAPIBase() string {
	proxy := strings.TrimSpace(c.Telegram.APIProxy)
	if proxy == "" {
		return "https://api.telegram.org"
	}
	return strings.TrimRight(proxy, "/")
}

func normalize(cfg *Config) bool {
	changed := false
	if strings.TrimSpace(cfg.App.Host) == "" {
		cfg.App.Host = "127.0.0.1"
		changed = true
	}
	if cfg.App.Port == 0 {
		cfg.App.Port = 8787
		changed = true
	}
	if strings.TrimSpace(cfg.Telegram.CommandMode) == "" {
		cfg.Telegram.CommandMode = "polling"
		changed = true
	}
	if strings.TrimSpace(cfg.Database.Driver) == "" {
		cfg.Database.Driver = "sqlite"
		changed = true
	}
	if strings.TrimSpace(cfg.Database.DSN) == "" {
		cfg.Database.DSN = defaultDatabaseDSN()
		changed = true
	}
	if strings.TrimSpace(cfg.Security.SessionSecret) == "" {
		cfg.Security.SessionSecret = randomHex(32)
		changed = true
	}
	if strings.TrimSpace(cfg.Security.TokenHashSecret) == "" {
		cfg.Security.TokenHashSecret = randomHex(32)
		changed = true
	}
	if strings.TrimSpace(cfg.Log.Level) == "" {
		cfg.Log.Level = "info"
		changed = true
	}
	return changed
}

func writeTemplate(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content := fmt.Sprintf(`# Hookgram 配置文件
# 程序首次启动时会自动生成本文件；修改后部分基础项可在管理端热更新。

app:
  # HTTP 监听地址，默认仅允许本机访问。
  host: %q
  # HTTP 监听端口。
  port: %d
  # 对外访问根地址；留空时默认使用 http://localhost:{port}。
  base_url: %q

telegram:
  # Telegram Bot Token；管理端初始化或设置页可修改。
  bot_token: %q
  # Telegram Bot API 代理地址；留空时直连 https://api.telegram.org。
  api_proxy: %q
  # Bot 命令接收模式；MVP 默认使用 long polling。
  command_mode: %q

database:
  # 数据库类型：sqlite、mysql、mariadb、postgres、postgresql。
  driver: %q
  # 数据库连接字符串；SQLite 默认写入 data/hookgram.db。
  dsn: %q

security:
  # 管理端 Session 签名密钥；自动生成，请勿泄露。
  session_secret: %q
  # Webhook Token 哈希密钥；自动生成，修改会导致已有 Token 失效。
  token_hash_secret: %q

log:
  # 日志级别：debug、info、warn、error。
  level: %q
`,
		cfg.App.Host,
		cfg.App.Port,
		cfg.App.BaseURL,
		cfg.Telegram.BotToken,
		cfg.Telegram.APIProxy,
		cfg.Telegram.CommandMode,
		cfg.Database.Driver,
		cfg.Database.DSN,
		cfg.Security.SessionSecret,
		cfg.Security.TokenHashSecret,
		cfg.Log.Level,
	)
	return os.WriteFile(path, []byte(content), 0600)
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

func dataDirFromEnv() string {
	if dir := strings.TrimSpace(os.Getenv(EnvDataDir)); dir != "" {
		return dir
	}
	return DefaultDataDir
}

func defaultDatabaseDSN() string {
	return filepath.Join(dataDirFromEnv(), "hookgram.db")
}
