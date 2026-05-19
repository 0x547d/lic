package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// JWTClaims 自定义 JWT Claims
type JWTClaims struct {
	UserID     uint   `json:"user_id"`
	Username   string `json:"username"`
	LicenseID  uint   `json:"license_id,omitempty"`
	LicenseKey string `json:"license_key,omitempty"`
	jwt.RegisteredClaims
}

var (
	JWTSecret = []byte("change-me-in-production-use-env-var")
	JWTExpire = 24 * time.Hour
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
)

// InitRSA 初始化 RSA 密钥对（用于离线激活文件签名）
func InitRSA(privateKeyPath, publicKeyPath string) error {
	if privateKeyPath != "" {
		keyData, err := os.ReadFile(privateKeyPath)
		if err == nil {
			block, _ := pem.Decode(keyData)
			if block != nil {
				key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
				if err == nil {
					privateKey = key
					publicKey = &key.PublicKey
					return nil
				}
			}
		}
	}
	var err error
	privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	publicKey = &privateKey.PublicKey
	return nil
}

// GetPublicKeyPEM 获取公钥 PEM 格式字符串
func GetPublicKeyPEM() string {
	if publicKey == nil {
		return ""
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return ""
	}
	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}
	return string(pem.EncodeToMemory(block))
}

// GenerateJWT 生成 JWT Token
func GenerateJWT(userID uint, username, licenseKey string, licenseID uint) (string, error) {
	claims := JWTClaims{
		UserID:      userID,
		Username:    username,
		LicenseID:   licenseID,
		LicenseKey:  licenseKey,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(JWTExpire)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// ParseJWT 解析 JWT Token
func ParseJWT(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return JWTSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrInvalidKey
	}
	return claims, nil
}

// HashPassword 密码哈希
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateRequestToken 生成离线激活请求令牌
func GenerateRequestToken() string {
	return uuid.New().String()
}
