package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Listen string `yaml:"listen"`
	} `yaml:"server"`
	Paths struct {
		Roms   string `yaml:"roms"`
		Data   string `yaml:"data"`
		Config string `yaml:"config"`
	} `yaml:"paths"`
	SQLite struct {
		Path string `yaml:"path"`
	} `yaml:"sqlite"`
	ScreenScraper struct {
		DevID       string `yaml:"dev_id"`
		DevPassword string `yaml:"dev_password"`
		UserSS      string `yaml:"user_ss"`
		UserPass    string `yaml:"user_password"`
		Softname    string `yaml:"softname"`
	} `yaml:"screenscraper"`
	IGDB struct {
		ClientID     string `yaml:"client_id"`
		ClientSecret string `yaml:"client_secret"`
	} `yaml:"igdb"`
	SteamGridDB struct {
		APIKey string `yaml:"api_key"`
	} `yaml:"steamgriddb"`
	LLM struct {
		BaseURL string `yaml:"base_url"`
		APIKey  string `yaml:"api_key"`
		Model   string `yaml:"model"`
	} `yaml:"llm"`
	Scan struct {
		RehashOnMtimeSizeChange bool `yaml:"rehash_on_mtime_size_change"`
		FollowSymlinks          bool `yaml:"follow_symlinks"`
	} `yaml:"scan"`
	Export struct {
		Root string `yaml:"root"`
	} `yaml:"export"`
	Security struct {
		AllowROMRename bool `yaml:"allow_rom_rename"`
	} `yaml:"security"`
	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
}

func ConfigPath() string {
	if path := os.Getenv("RETROVE_CONFIG"); path != "" {
		return path
	}
	return "app/config/config.yaml"
}

func Load() (Config, error) {
	cfg := defaults()
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("read %s: %w", path, err)
		}
	} else if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	applyEnv(&cfg)
	normalize(&cfg)
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

type ScreenScraperInput struct {
	DevID       *string `json:"dev_id"`
	DevPassword *string `json:"dev_password"`
	UserSS      *string `json:"user_ss"`
	UserPass    *string `json:"user_password"`
	Softname    *string `json:"softname"`
}

type IGDBInput struct {
	ClientID     *string `json:"client_id"`
	ClientSecret *string `json:"client_secret"`
}

type SteamGridDBInput struct {
	APIKey *string `json:"api_key"`
}

type LLMInput struct {
	BaseURL *string `json:"base_url"`
	APIKey  *string `json:"api_key"`
	Model   *string `json:"model"`
}

func (c *Config) ApplyScreenScraper(in ScreenScraperInput) {
	if in.DevID != nil {
		c.ScreenScraper.DevID = strings.TrimSpace(*in.DevID)
	}
	if in.DevPassword != nil {
		c.ScreenScraper.DevPassword = *in.DevPassword
	}
	if in.UserSS != nil {
		c.ScreenScraper.UserSS = strings.TrimSpace(*in.UserSS)
	}
	if in.UserPass != nil {
		c.ScreenScraper.UserPass = *in.UserPass
	}
	if in.Softname != nil {
		c.ScreenScraper.Softname = strings.TrimSpace(*in.Softname)
		if c.ScreenScraper.Softname == "" {
			c.ScreenScraper.Softname = "Retrove"
		}
	}
}

func (c *Config) ApplyIGDB(in IGDBInput) {
	if in.ClientID != nil {
		c.IGDB.ClientID = strings.TrimSpace(*in.ClientID)
	}
	if in.ClientSecret != nil {
		c.IGDB.ClientSecret = *in.ClientSecret
	}
}

func (c *Config) ApplySteamGridDB(in SteamGridDBInput) {
	if in.APIKey != nil {
		c.SteamGridDB.APIKey = strings.TrimSpace(*in.APIKey)
	}
}

func (c *Config) ApplyLLM(in LLMInput) {
	if in.BaseURL != nil {
		c.LLM.BaseURL = strings.TrimRight(strings.TrimSpace(*in.BaseURL), "/")
	}
	if in.APIKey != nil {
		c.LLM.APIKey = strings.TrimSpace(*in.APIKey)
	}
	if in.Model != nil {
		c.LLM.Model = strings.TrimSpace(*in.Model)
	}
}

func (c Config) ScreenScraperConfigured() bool {
	return strings.TrimSpace(c.ScreenScraper.DevID) != "" && strings.TrimSpace(c.ScreenScraper.DevPassword) != ""
}

func (c Config) IGDBConfigured() bool {
	return strings.TrimSpace(c.IGDB.ClientID) != "" && strings.TrimSpace(c.IGDB.ClientSecret) != ""
}

func (c Config) SteamGridDBConfigured() bool {
	return strings.TrimSpace(c.SteamGridDB.APIKey) != ""
}

func (c Config) LLMConfigured() bool {
	return strings.TrimSpace(c.LLM.BaseURL) != "" &&
		strings.TrimSpace(c.LLM.APIKey) != "" &&
		strings.TrimSpace(c.LLM.Model) != ""
}

func (c Config) AnyScrapeSourceConfigured() bool {
	return c.ScreenScraperConfigured() || c.IGDBConfigured() || c.SteamGridDBConfigured()
}

func (c Config) Save() error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir config: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (c Config) EnsureDirs() error {
	dirs := []string{
		c.Paths.Data,
		filepath.Join(c.Paths.Data, "media"),
		filepath.Join(c.Paths.Data, "export"),
		filepath.Join(c.Paths.Data, "cache"),
		filepath.Join(c.Paths.Config, "dictionaries"),
		filepath.Dir(c.SQLite.Path),
		c.Export.Root,
	}
	for _, d := range dirs {
		if d == "" || d == "." {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	return nil
}

func defaults() Config {
	var cfg Config
	cfg.Server.Listen = ":5080"
	cfg.Paths.Roms = "/app/roms"
	cfg.Paths.Data = "/app/data"
	cfg.Paths.Config = "/app/config"
	cfg.SQLite.Path = "/app/data/retrove.db"
	cfg.Export.Root = "/app/data/export"
	cfg.ScreenScraper.Softname = "Retrove"
	cfg.Scan.RehashOnMtimeSizeChange = true
	cfg.Log.Level = "info"
	return cfg
}

func applyEnv(cfg *Config) {
	set := func(key string, dest *string) {
		if v := os.Getenv(key); v != "" {
			*dest = v
		}
	}
	set("RETROVE_LISTEN", &cfg.Server.Listen)
	set("RETROVE_ROMS", &cfg.Paths.Roms)
	set("RETROVE_DATA", &cfg.Paths.Data)
	set("RETROVE_CONFIG_DIR", &cfg.Paths.Config)
	set("RETROVE_SQLITE", &cfg.SQLite.Path)
	set("RETROVE_EXPORT", &cfg.Export.Root)
	set("RETROVE_LOG_LEVEL", &cfg.Log.Level)
	set("RETROVE_SS_DEV_ID", &cfg.ScreenScraper.DevID)
	set("RETROVE_SS_DEV_PASSWORD", &cfg.ScreenScraper.DevPassword)
	set("RETROVE_SS_USER", &cfg.ScreenScraper.UserSS)
	set("RETROVE_SS_USER_PASSWORD", &cfg.ScreenScraper.UserPass)
	set("RETROVE_IGDB_CLIENT_ID", &cfg.IGDB.ClientID)
	set("RETROVE_IGDB_CLIENT_SECRET", &cfg.IGDB.ClientSecret)
	set("RETROVE_STEAMGRIDDB_API_KEY", &cfg.SteamGridDB.APIKey)
	set("RETROVE_LLM_BASE_URL", &cfg.LLM.BaseURL)
	set("RETROVE_LLM_API_KEY", &cfg.LLM.APIKey)
	set("RETROVE_LLM_MODEL", &cfg.LLM.Model)
}

func normalize(cfg *Config) {
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":5080"
	}
	if !strings.Contains(cfg.Server.Listen, ":") {
		cfg.Server.Listen = ":" + cfg.Server.Listen
	}
	if cfg.SQLite.Path == "" {
		cfg.SQLite.Path = filepath.Join(cfg.Paths.Data, "retrove.db")
	}
	if cfg.Export.Root == "" {
		cfg.Export.Root = filepath.Join(cfg.Paths.Data, "export")
	}
	if cfg.ScreenScraper.Softname == "" {
		cfg.ScreenScraper.Softname = "Retrove"
	}
	if cfg.LLM.BaseURL != "" {
		cfg.LLM.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.LLM.BaseURL), "/")
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
}

func validate(cfg Config) error {
	if cfg.Paths.Roms == "" || cfg.Paths.Data == "" || cfg.Paths.Config == "" {
		return fmt.Errorf("paths.roms / paths.data / paths.config 不能为空")
	}
	if cfg.SQLite.Path == "" {
		return fmt.Errorf("sqlite.path 不能为空")
	}
	return nil
}
