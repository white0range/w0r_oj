package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	App           AppInfoConfig       `mapstructure:"app"`
	Server        ServerConfig        `mapstructure:"server"`
	SQL           SQLConfig           `mapstructure:"sql"`
	Redis         RedisConfig         `mapstructure:"redis"`
	JWT           JWTConfig           `mapstructure:"jwt"`
	AI            AIConfig            `mapstructure:"ai"`
	Elasticsearch ElasticsearchConfig `mapstructure:"elasticsearch"`
	Judge         JudgeConfig         `mapstructure:"judge"`
	Chat          ChatConfig          `mapstructure:"chat"`
	RateLimit     RateLimitConfig     `mapstructure:"rate_limit"`
}

type AppInfoConfig struct {
	Env string `mapstructure:"env"`
}

type ServerConfig struct {
	Port                     int      `mapstructure:"port"`
	TrustedProxyCIDRs        []string `mapstructure:"trusted_proxy_cidrs"`
	ReadHeaderTimeoutSeconds int      `mapstructure:"read_header_timeout_seconds"`
	ReadTimeoutSeconds       int      `mapstructure:"read_timeout_seconds"`
	WriteTimeoutSeconds      int      `mapstructure:"write_timeout_seconds"`
	IdleTimeoutSeconds       int      `mapstructure:"idle_timeout_seconds"`
	MaxHeaderBytes           int      `mapstructure:"max_header_bytes"`
	MaxRequestBodyBytes      int64    `mapstructure:"max_request_body_bytes"`
}

type SQLConfig struct {
	Dsn                    string
	MaxOpenConns           int `mapstructure:"max_open_conns"`
	MaxIdleConns           int `mapstructure:"max_idle_conns"`
	ConnMaxLifetimeSeconds int `mapstructure:"conn_max_lifetime_seconds"`
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret           string
	AccessTTLMinutes int `mapstructure:"access_ttl_minutes"`
	RefreshTTLHours  int `mapstructure:"refresh_ttl_hours"`
}

type AIConfig struct {
	APIKey         string `mapstructure:"api_key"`
	BaseURL        string `mapstructure:"base_url"`
	Model          string `mapstructure:"model"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"`
}

type ElasticsearchConfig struct {
	Addresses []string `mapstructure:"addresses"`
}

type JudgeConfig struct {
	WorkerCount           int `mapstructure:"worker_count"`
	CompileTimeoutSeconds int `mapstructure:"compile_timeout_seconds"`
}

type RateLimitConfig struct {
	PublicIPPerMinute             int `mapstructure:"public_ip_per_minute"`
	LoginIPPerMinute              int `mapstructure:"login_ip_per_minute"`
	LoginAccountPerMinute         int `mapstructure:"login_account_per_minute"`
	RegisterIPPerHour             int `mapstructure:"register_ip_per_hour"`
	SuccessfulRegistrationsPerDay int `mapstructure:"successful_registrations_per_day"`
	SubmitPerFiveSeconds          int `mapstructure:"submit_per_five_seconds"`
	SubmitPerHour                 int `mapstructure:"submit_per_hour"`
	ChatMessagesPerDay            int `mapstructure:"chat_messages_per_day"`
	SearchIPPerMinute             int `mapstructure:"search_ip_per_minute"`
	SSETicketsPerMinute           int `mapstructure:"sse_tickets_per_minute"`
	SSEConnectionsPerUser         int `mapstructure:"sse_connections_per_user"`
}

type ChatConfig struct {
	WorkerCount         int    `mapstructure:"worker_count"`
	AgentBaseURL        string `mapstructure:"agent_base_url"`
	AgentTimeoutSeconds int    `mapstructure:"agent_timeout_seconds"`
	AgentServiceToken   string `mapstructure:"agent_service_token"`
}

var GlobalConfig Config

func InitConfig() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.SetConfigName("config." + env)

	viper.SetDefault("app.env", env)
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.trusted_proxy_cidrs", []string(nil))
	viper.SetDefault("server.read_header_timeout_seconds", 5)
	viper.SetDefault("server.read_timeout_seconds", 15)
	// Chat uses SSE, so response deadlines are managed by the proxy and the
	// stream heartbeat instead of a global Go write timeout.
	viper.SetDefault("server.write_timeout_seconds", 0)
	viper.SetDefault("server.idle_timeout_seconds", 60)
	viper.SetDefault("server.max_header_bytes", 1<<20)
	viper.SetDefault("server.max_request_body_bytes", int64(1<<20))

	viper.SetDefault("sql.max_open_conns", 20)
	viper.SetDefault("sql.max_idle_conns", 10)
	viper.SetDefault("sql.conn_max_lifetime_seconds", 3600)

	viper.SetDefault("ai.base_url", "https://api.deepseek.com")
	viper.SetDefault("ai.model", "deepseek-chat")
	viper.SetDefault("ai.timeout_seconds", 30)

	viper.SetDefault("elasticsearch.addresses", []string{"http://localhost:9200"})
	viper.SetDefault("judge.worker_count", 3)
	viper.SetDefault("judge.compile_timeout_seconds", 45)
	viper.SetDefault("chat.worker_count", 3)
	viper.SetDefault("chat.agent_base_url", "http://localhost:8000")
	viper.SetDefault("chat.agent_timeout_seconds", 120)
	viper.SetDefault("jwt.access_ttl_minutes", 120)
	viper.SetDefault("jwt.refresh_ttl_hours", 168)
	viper.SetDefault("rate_limit.public_ip_per_minute", 300)
	viper.SetDefault("rate_limit.login_ip_per_minute", 10)
	viper.SetDefault("rate_limit.login_account_per_minute", 5)
	viper.SetDefault("rate_limit.register_ip_per_hour", 3)
	viper.SetDefault("rate_limit.successful_registrations_per_day", 10)
	viper.SetDefault("rate_limit.submit_per_five_seconds", 1)
	viper.SetDefault("rate_limit.submit_per_hour", 60)
	viper.SetDefault("rate_limit.chat_messages_per_day", 2)
	viper.SetDefault("rate_limit.search_ip_per_minute", 60)
	viper.SetDefault("rate_limit.sse_tickets_per_minute", 20)
	viper.SetDefault("rate_limit.sse_connections_per_user", 3)

	viper.SetEnvPrefix("GOJO")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("read config file failed: %v", err)
	}

	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		log.Fatalf("unmarshal config failed: %v", err)
	}

	if err := ValidateStartupConfig(GlobalConfig, env); err != nil {
		log.Fatalf("invalid production security configuration: %v", err)
	}

	fmt.Println("system config loaded successfully")
}

const minProductionSecretLength = 32

// ValidateStartupConfig rejects secrets that would make a production
// deployment forgeable or leave internal services unauthenticated.
func ValidateStartupConfig(cfg Config, selectedEnv string) error {
	if !isProductionEnv(selectedEnv) && !isProductionEnv(cfg.App.Env) {
		return nil
	}

	checks := []struct {
		name  string
		value string
	}{
		{"jwt.secret", cfg.JWT.Secret},
		{"chat.agent_service_token", cfg.Chat.AgentServiceToken},
		{"redis.password", cfg.Redis.Password},
	}
	for _, check := range checks {
		if isUnsafeProductionSecret(check.value) {
			return fmt.Errorf("%s must be a random secret of at least %d characters and must not use a sample value", check.name, minProductionSecretLength)
		}
	}
	return nil
}

func isProductionEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

func isUnsafeProductionSecret(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < minProductionSecretLength {
		return true
	}

	lower := strings.ToLower(value)
	return strings.Contains(lower, "replace-with") ||
		strings.Contains(lower, "change-me") ||
		lower == "secret" || lower == "password"
}
