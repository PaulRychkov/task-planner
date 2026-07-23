package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	HTTPPort       int
	DB             DBConfig
	KafkaBrokers   []string
	KafkaTopic     string
	Timezone       string
	WindowDays     int
	OutboxInterval time.Duration
	Location       *time.Location
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=10",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

func Load() (Config, error) {
	loadDotEnv(".env")
	loadDotEnv("../.env")

	v := viper.New()
	v.SetEnvPrefix("TASKS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("http_port", 8081)
	v.SetDefault("db_host", "localhost")
	v.SetDefault("db_port", 5433)
	v.SetDefault("db_user", "tasks")
	v.SetDefault("db_password", "tasks")
	v.SetDefault("db_name", "tasks")
	v.SetDefault("db_sslmode", "disable")
	v.SetDefault("kafka_brokers", "localhost:9094")
	v.SetDefault("kafka_topic", "tasks.events")
	v.SetDefault("timezone", "Europe/Moscow")
	v.SetDefault("window_days", 60)
	v.SetDefault("outbox_interval_seconds", 5)

	cfg := Config{
		HTTPPort: v.GetInt("http_port"),
		DB: DBConfig{
			Host:     v.GetString("db_host"),
			Port:     v.GetInt("db_port"),
			User:     v.GetString("db_user"),
			Password: v.GetString("db_password"),
			Name:     v.GetString("db_name"),
			SSLMode:  v.GetString("db_sslmode"),
		},
		KafkaBrokers:   splitList(v.GetString("kafka_brokers")),
		KafkaTopic:     v.GetString("kafka_topic"),
		Timezone:       v.GetString("timezone"),
		WindowDays:     v.GetInt("window_days"),
		OutboxInterval: time.Duration(v.GetInt("outbox_interval_seconds")) * time.Second,
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return Config{}, fmt.Errorf("load timezone %q: %w", cfg.Timezone, err)
	}
	cfg.Location = loc

	if cfg.WindowDays < 1 {
		return Config{}, fmt.Errorf("window_days must be positive, got %d", cfg.WindowDays)
	}

	return cfg, nil
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
