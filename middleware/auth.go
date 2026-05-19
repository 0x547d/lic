package middleware

import (
	"net/http"
	"strings"

	"license-server/models"
	"license-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// JWTAuth JWT 认证中间件
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		claims, err := utils.ParseJWT(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		// 将 claims 存入上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("license_id", claims.LicenseID)
		c.Set("license_key", claims.LicenseKey)
		c.Next()
	}
}

// LicenseAuth 授权码验证中间件（检查授权是否有效）
func LicenseAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		licenseKey, ok := c.Get("license_key")
		if !ok || licenseKey == "" {
			// 尝试从请求体或查询参数获取
			lk := c.Query("license_key")
			if lk == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing license key"})
				return
			}
			licenseKey = lk
		}

		var license models.License
		if err := db.Where("license_key = ?", licenseKey).First(&license).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "license not found"})
			return
		}

		if !license.IsValid() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":    "license is not valid",
				"status":   string(license.Status),
				"valid_to": license.ValidTo,
			})
			return
		}

		c.Set("license_status", license.Status)
		c.Set("license_valid_to", license.ValidTo)
		c.Next()
	}
}
