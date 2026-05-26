package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// 定义配置结构体，与 yaml 文件的层级一一对应
type Config struct {
	App           AppInfoConfig
	Server        ServerConfig
	SQL           SQLConfig
	Redis         RedisConfig
	JWT           JWTConfig
	AI            AIConfig
	Elasticsearch ElasticsearchConfig
	Judge         JudgeConfig
}

// 项目运行环境，比如 dev / test / prod
type AppInfoConfig struct {
	Env string `mapstructure:"env"`
}

// HTTP 服务自己的配置
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

// Elasticsearch 配置
type ElasticsearchConfig struct {
	Addresses []string `mapstructure:"addresses"`
}

// 判题 worker 配置
type JudgeConfig struct {
	WorkerCount int `mapstructure:"worker_count"`
}

// 全局配置变量。
// 注意：这里我叫它 GlobalConfig，避免和上面的类型名冲突。
var GlobalConfig Config

// InitConfig 初始化加载配置
func InitConfig() {
	// 1. 先看有没有通过环境变量指定运行环境
	// 如果没有，就默认走 dev
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	//viper.SetConfigName("config")   // 配置文件名称(无扩展名)
	viper.SetConfigType("yaml")     // 文件类型
	viper.AddConfigPath("./config") // 告诉 viper 去哪里找这个文件
	// 3. 关键：根据环境拼配置文件名
	// dev  -> config.dev.yaml
	// test -> config.test.yaml
	// prod -> config.prod.yaml
	viper.SetConfigName("config." + env)

	// 3. 给一些配置设置默认值
	// 这样即使 yaml 里没写，也不会直接是零值
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

	// 4. 支持环境变量覆盖配置
	// 比如 GOJO_SERVER_PORT=9090
	viper.SetEnvPrefix("GOJO")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 1. 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("❌ 致命错误：读取配置文件失败: %v", err)
	}

	// 2. 把读取到的配置反序列化到结构体里
	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		log.Fatalf("❌ 致命错误：解析配置文件失败: %v", err)
	}

	fmt.Println("⚙️  系统配置加载成功！")
}
