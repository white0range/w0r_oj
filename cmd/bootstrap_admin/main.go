package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode/utf8"

	"gojo/config"
	"gojo/infrastructure/mysql"
	userModel "gojo/internal/user/model"
	passwordutil "gojo/pkg/password"

	"gorm.io/gorm"
)

const (
	usernameEnv = "BOOTSTRAP_ADMIN_USERNAME"
	passwordEnv = "BOOTSTRAP_ADMIN_PASSWORD"
)

func main() {
	config.InitConfig()
	mysql.InitDB()
	defer func() {
		if err := mysql.Close(); err != nil {
			log.Printf("close MySQL: %v", err)
		}
	}()

	username := strings.TrimSpace(os.Getenv(usernameEnv))
	plainPassword := os.Getenv(passwordEnv)
	if err := validateCredentials(username, plainPassword); err != nil {
		log.Fatal(err)
	}

	var existing userModel.User
	err := mysql.DB.Where("username = ?", username).First(&existing).Error
	switch {
	case err == nil:
		if existing.Role == 1 && existing.Status == userModel.UserStatusActive && existing.TokenVersion > 0 {
			fmt.Printf("admin %q is already configured\n", username)
			return
		}
		log.Fatalf("user %q already exists but is not an active administrator", username)
	case !errors.Is(err, gorm.ErrRecordNotFound):
		log.Fatalf("check existing administrator: %v", err)
	}

	hash, err := passwordutil.HashPassword(plainPassword)
	if err != nil {
		log.Fatalf("hash administrator password: %v", err)
	}

	admin := userModel.User{
		Username:     username,
		Password:     hash,
		Role:         1,
		Status:       userModel.UserStatusActive,
		TokenVersion: 1,
	}
	if err := mysql.DB.Create(&admin).Error; err != nil {
		log.Fatalf("create administrator: %v", err)
	}

	fmt.Printf("created administrator %q (id=%d)\n", username, admin.ID)
}

func validateCredentials(username, plainPassword string) error {
	usernameLength := utf8.RuneCountInString(username)
	if usernameLength < 3 || usernameLength > 32 {
		return fmt.Errorf("%s must contain 3-32 characters", usernameEnv)
	}
	passwordLength := len([]byte(plainPassword))
	if passwordLength < 12 || passwordLength > 72 {
		return fmt.Errorf("%s must contain 12-72 bytes", passwordEnv)
	}
	return nil
}
