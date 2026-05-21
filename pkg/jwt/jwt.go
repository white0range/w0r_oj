package jwt

import (
	"errors"
	"gojo/config"
	"gojo/internal/user/model"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 公司的最高机密：签名密钥！
// 绝对不能泄露，如果有黑客拿到了这串乱码，他就能自己伪造咱们 OJ 平台的手环了
// var jwtSecret = []byte(config.AppConfig.JWT.Secret)
func getJWTSecret() []byte {
	return []byte(config.AppConfig.JWT.Secret)
}

// GenerateToken 负责为登录成功的用户生成专属手环
// 传入用户的 ID 和用户名，把它们封印在手环里
func GenerateToken(user *model.User) (string, error) {
	// 1. 创建手环里面的数据载体（Payload）
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		// 设置手环的过期时间，比如 24 小时后失效
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}

	// 2. 使用 HS256 算法生成手环的模具
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 3. 用咱们公司的最高机密盖章，生成最终发给用户的字符串
	return token.SignedString(getJWTSecret())
}

// ParseToken 负责验证手环的真伪，并把里面的数据（Payload）提取出来
// 凭空捏造一个 error 对象，里面装着咱们自定义的汉字，然后把它 return 出去
// return nil, errors.New("无效的手环")
func ParseToken(tokenString string) (*jwt.MapClaims, error) {
	// 1. 解析并校验 Token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 把咱们公司的绝密印章提供给解析器，用来比对签名
		return getJWTSecret(), nil
	})

	// 2. 如果解析失败，或者手环过期了、被篡改了
	if err != nil || !token.Valid {
		return nil, errors.New("无效的手环")
	}

	// 3. 把手环里的数据（咱们之前塞进去的 Map）拿出来
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return &claims, nil
	}

	return nil, errors.New("无法提取手环数据")
}
