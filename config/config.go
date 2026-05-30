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
	StudyPlan     StudyPlanConfig     `mapstructure:"study_plan"`
}

type AppInfoConfig struct {
	Env string `mapstructure:"env"`
}

type ServerConfig struct {
	Port                int `mapstructure:"port"`
	ReadTimeoutSeconds  int `mapstructure:"read_timeout_seconds"`
	WriteTimeoutSeconds int `mapstructure:"write_timeout_seconds"`
	IdleTimeoutSeconds  int `mapstructure:"idle_timeout_seconds"`
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
	Secret string
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
	WorkerCount int `mapstructure:"worker_count"`
}

type StudyPlanConfig struct {
	WorkerCount         int    `mapstructure:"worker_count"`
	AgentBaseURL        string `mapstructure:"agent_base_url"`
	AgentTimeoutSeconds int    `mapstructure:"agent_timeout_seconds"`
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
	viper.SetDefault("server.read_timeout_seconds", 10)
	viper.SetDefault("server.write_timeout_seconds", 10)
	viper.SetDefault("server.idle_timeout_seconds", 60)

	viper.SetDefault("sql.max_open_conns", 20)
	viper.SetDefault("sql.max_idle_conns", 10)
	viper.SetDefault("sql.conn_max_lifetime_seconds", 3600)

	viper.SetDefault("ai.base_url", "https://api.deepseek.com")
	viper.SetDefault("ai.model", "deepseek-chat")
	viper.SetDefault("ai.timeout_seconds", 30)

	viper.SetDefault("elasticsearch.addresses", []string{"http://localhost:9200"})
	viper.SetDefault("judge.worker_count", 3)
	viper.SetDefault("study_plan.worker_count", 3)
	viper.SetDefault("study_plan.agent_base_url", "http://localhost:8000")
	viper.SetDefault("study_plan.agent_timeout_seconds", 60)

	viper.SetEnvPrefix("GOJO")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("read config file failed: %v", err)
	}

	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		log.Fatalf("unmarshal config failed: %v", err)
	}

	fmt.Println("system config loaded successfully")
}
