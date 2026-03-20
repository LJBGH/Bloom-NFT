package middleware

import (
	"net/http"
	"strings"
	"time"

	"bloom-nft/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTSecret 从环境变量读取，未设置则使用一个默认值（仅用于开发环境）
func getJWTSecret() []byte {
	return []byte(config.AppConfig.JWT.Secret)
}

// GenerateToken 生成 JWT，示例只包含 userID 和过期时间
func GenerateToken(userID uint) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"iss":     config.AppConfig.JWT.Issuer,                                                       // 发行人
		"sub":     config.AppConfig.JWT.Subject,                                                      // 主题
		"aud":     config.AppConfig.JWT.Audience,                                                     // 受众
		"alg":     config.AppConfig.JWT.Algorithm,                                                    // 算法
		"exp":     time.Now().Add(time.Duration(config.AppConfig.JWT.ExpireTime) * time.Hour).Unix(), // 过期时间
		"iat":     time.Now().Unix(),                                                                 // 创建时间
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims) // 使用 HS256 算法生成 JWT
	return token.SignedString(getJWTSecret())
}

// AuthMiddleware JWT 认证中间件
// 从 Authorization: Bearer <token> 头中解析 JWT，验证失败则返回 401
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header is required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			return
		}

		tokenStr := parts[1]

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return getJWTSecret(), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			// 示例：把 user_id 放到上下文中，方便后续 handler 使用
			if uid, ok2 := claims["user_id"]; ok2 {
				c.Set("user_id", uid)
			}
		}

		c.Next()
	}
}
