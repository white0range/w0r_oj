package config

import "testing"

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
