package config

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestBoundEnvironmentIsUnmarshaled(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	const secret = "0123456789abcdef0123456789abcdef"
	t.Setenv("GOJO_JWT_SECRET", secret)
	viper.SetEnvPrefix("GOJO")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	if err := bindEnvironment(); err != nil {
		t.Fatalf("bindEnvironment() error = %v", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if cfg.JWT.Secret != secret {
		t.Fatalf("JWT secret was not loaded from GOJO_JWT_SECRET")
	}
}

func TestValidateStartupConfig(t *testing.T) {
	validSecret := "0123456789abcdef0123456789abcdef"
	base := Config{
		App:   AppInfoConfig{Env: "prod"},
		JWT:   JWTConfig{Secret: validSecret},
		Chat:  ChatConfig{AgentServiceToken: validSecret},
		Redis: RedisConfig{Password: validSecret},
	}

	if err := ValidateStartupConfig(base, "prod"); err != nil {
		t.Fatalf("valid production config rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Config)
	}{
		{"empty jwt secret", func(c *Config) { c.JWT.Secret = "" }},
		{"short agent token", func(c *Config) { c.Chat.AgentServiceToken = "too-short" }},
		{"sample redis password", func(c *Config) { c.Redis.Password = "replace-with-a-long-random-password" }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			tt.mutate(&cfg)
			if err := ValidateStartupConfig(cfg, "prod"); err == nil {
				t.Fatal("unsafe production secret was accepted")
			}
		})
	}

	dev := base
	dev.App.Env = "dev"
	dev.JWT.Secret = ""
	dev.Chat.AgentServiceToken = ""
	dev.Redis.Password = ""
	if err := ValidateStartupConfig(dev, "dev"); err != nil {
		t.Fatalf("development config should remain usable: %v", err)
	}
}
