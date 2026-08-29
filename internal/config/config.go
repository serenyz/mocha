package config

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	value, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", text, err)
	}
	d.Duration = value
	return nil
}

type MainConfig struct {
	AppName string `toml:"appName"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
}

type MysqlConfig struct {
	Host            string `toml:"host"`
	Port            int    `toml:"port"`
	User            string `toml:"user"`
	Password        string `toml:"password"`
	DatabaseName    string `toml:"databaseName"`
	MaxIdleConns    int    `toml:"maxIdleConns"`
	MaxOpenConns    int    `toml:"maxOpenConns"`
	ConnMaxLifetime int    `toml:"connMaxLifetime"`
}

type LogConfig struct {
	Path       string `toml:"path"`
	Level      string `toml:"level"`
	MaxSize    int    `toml:"maxSize"`
	MaxBackups int    `toml:"maxBackups"`
	MaxAge     int    `toml:"maxAge"`
}

type StaticSrcConfig struct {
	StaticAvatarPath string `toml:"staticAvatarPath"`
	StaticFilePath   string `toml:"staticFilePath"`
}

type RedisConfig struct {
	Host         string `toml:"host"`
	Port         int    `toml:"port"`
	Password     string `toml:"password"`
	DB           int    `toml:"db"`
	PoolSize     int    `toml:"poolSize"`
	MinIdleConns int    `toml:"minIdleConns"`
}

type AuthConfig struct {
	Issuer                       string   `toml:"issuer"`
	AccessTokenTTL               Duration `toml:"accessTokenTTL"`
	RefreshTokenTTL              Duration `toml:"refreshTokenTTL"`
	RegisterCodeTTL              Duration `toml:"registerCodeTTL"`
	RegisterCodeResendInterval   Duration `toml:"registerCodeResendInterval"`
	RegisterCodePhoneHourlyLimit int      `toml:"registerCodePhoneHourlyLimit"`
}

type MinIOConfig struct {
	Endpoint string `toml:"endpoint"`
	Bucket   string `toml:"bucket"`
	Region   string `toml:"region"`
	UseSSL   bool   `toml:"useSSL"`
}

type Config struct {
	MainConfig      `toml:"mainConfig"`
	MysqlConfig     `toml:"mysqlConfig"`
	LogConfig       `toml:"logConfig"`
	StaticSrcConfig `toml:"staticSrcConfig"`
	RedisConfig     `toml:"redisConfig"`
	AuthConfig      `toml:"authConfig"`
	MinIOConfig     `toml:"minioConfig"`
}

var config *Config
var once sync.Once

func GetConfig() *Config {
	if config == nil {
		once.Do(func() {
			cfg, err := load("./configs/config.toml")
			if err != nil {
				log.Fatal("cannot init config: ", err)
			}
			config = cfg
		})
	}
	return config
}

func load(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
