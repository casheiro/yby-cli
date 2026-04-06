// Package config gerencia a configuração global do Yby CLI via ~/.yby/config.yaml.
// Precedência: flags > env vars > arquivo config > defaults.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	ybyerrors "github.com/casheiro/yby-cli/pkg/errors"
	"github.com/spf13/viper"
)

// RateLimitConfig armazena configuração de rate limiting para IA.
type RateLimitConfig struct {
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
}

// AIConfig armazena configuração do subsistema de IA.
type AIConfig struct {
	Provider  string          `mapstructure:"provider"`
	Model     string          `mapstructure:"model"`
	Language  string          `mapstructure:"language"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

// LogConfig armazena configuração de logging.
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// TelemetryConfig armazena configuração de telemetria.
type TelemetryConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// Config é a estrutura raiz de configuração global do Yby CLI.
type Config struct {
	AI        AIConfig        `mapstructure:"ai"`
	Log       LogConfig       `mapstructure:"log"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
}

var (
	globalConfig *Config
	once         sync.Once
)

// setDefaults aplica valores padrão no viper.
func setDefaults(v *viper.Viper) {
	v.SetDefault("ai.language", "pt-BR")
	v.SetDefault("ai.provider", "")
	v.SetDefault("ai.model", "")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("telemetry.enabled", false)
}

// bindEnvVars associa variáveis de ambiente ao viper com prefixo YBY.
func bindEnvVars(v *viper.Viper) {
	v.SetEnvPrefix("YBY")
	v.AutomaticEnv()

	// Bindings explícitos para variáveis com nomes específicos
	_ = v.BindEnv("ai.provider", "YBY_AI_PROVIDER")
	_ = v.BindEnv("ai.model", "YBY_AI_MODEL")
	_ = v.BindEnv("ai.language", "YBY_AI_LANGUAGE")
	_ = v.BindEnv("log.level", "YBY_LOG_LEVEL")
	_ = v.BindEnv("log.format", "YBY_LOG_FORMAT")
	_ = v.BindEnv("telemetry.enabled", "YBY_TELEMETRY_ENABLED")
}

// Load carrega a configuração global a partir de ~/.yby/config.yaml, env vars e defaults.
// Retorna a configuração carregada. Arquivo ausente não é erro — usa defaults.
func Load() (*Config, error) {
	v := viper.New()

	setDefaults(v)
	bindEnvVars(v)

	// Caminho do config: ~/.yby/config.yaml
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Debug("Não foi possível determinar o diretório home; usando apenas defaults e env vars", "erro", err)
	} else {
		configDir := filepath.Join(home, ".yby")
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(configDir)

		if err := v.ReadInConfig(); err != nil {
			// Arquivo ausente não é erro — segue com defaults
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				slog.Warn("Erro ao ler config", "caminho", filepath.Join(configDir, "config.yaml"), "erro", err)
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, ybyerrors.Wrap(err, ybyerrors.ErrCodeConfig, "falha ao deserializar configuração")
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate verifica se os valores de configuração são válidos.
func (c *Config) Validate() error {
	validProviders := map[string]bool{"": true, "ollama": true, "gemini": true, "openai": true}
	if !validProviders[c.AI.Provider] {
		return ybyerrors.New(ybyerrors.ErrCodeConfig,
			fmt.Sprintf("ai.provider inválido: %q (valores aceitos: ollama, gemini, openai)", c.AI.Provider))
	}

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Log.Level] {
		return ybyerrors.New(ybyerrors.ErrCodeConfig,
			fmt.Sprintf("log.level inválido: %q (valores aceitos: debug, info, warn, error)", c.Log.Level))
	}

	validFormats := map[string]bool{"text": true, "json": true}
	if !validFormats[c.Log.Format] {
		return ybyerrors.New(ybyerrors.ErrCodeConfig,
			fmt.Sprintf("log.format inválido: %q (valores aceitos: text, json)", c.Log.Format))
	}

	return nil
}

// LoadOnce carrega a configuração uma única vez (singleton thread-safe).
func LoadOnce() *Config {
	once.Do(func() {
		cfg, err := Load()
		if err != nil {
			slog.Error("Falha ao carregar configuração", "erro", err)
			cfg = DefaultConfig()
		}
		globalConfig = cfg
	})
	return globalConfig
}

// Get retorna a configuração global carregada. Se não foi carregada, chama LoadOnce.
func Get() *Config {
	if globalConfig == nil {
		return LoadOnce()
	}
	return globalConfig
}

// DefaultConfig retorna uma Config com todos os valores padrão.
func DefaultConfig() *Config {
	return &Config{
		AI: AIConfig{
			Language: "pt-BR",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
		Telemetry: TelemetryConfig{
			Enabled: false,
		},
	}
}

// ResetGlobal reseta o singleton global. Útil apenas para testes.
func ResetGlobal() {
	globalConfig = nil
	once = sync.Once{}
}
