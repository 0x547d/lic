package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/0x547d/lic/models"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	DBType     string // mysql 或 sqlite
	DBDSN      string // 数据库连接字符串
	HTTPAddr   string // HTTP 监听地址
	JWTSecret  string
	JWTExpire  time.Duration
	AdminUser  string // 管理员用户名（用于前端登录）
	AdminPass  string // 管理员密码（bcrypt 哈希）
	SMTPHost   string // SMTP 服务器地址
	SMTPPort   int    // SMTP 端口
	SMTPUser   string // SMTP 用户名
	SMTPPass   string // SMTP 密码
	SMTPFrom   string // 发件人地址
	AdminEmail string // 管理员收件邮箱（申请通知等）
}

func Load() *Config {
	cfg := &Config{
		DBType:     getEnv("DB_TYPE", "mysql"),
		DBDSN:      getEnv("DB_DSN", "root:password@tcp(127.0.0.1:3306)/license?charset=utf8mb4&parseTime=True&loc=Local"),
		HTTPAddr:   getEnv("HTTP_ADDR", ":8080"),
		JWTSecret:  getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpire:  24 * time.Hour,
		AdminUser:  getEnv("ADMIN_USER", "admin"),
		AdminPass:  getEnv("ADMIN_PASS_HASH", ""), // 通过环境变量设置 bcrypt 哈希
		SMTPHost:   getEnv("SMTP_HOST", ""),
		SMTPPort:   getEnvAsInt("SMTP_PORT", 587),
		SMTPUser:   getEnv("SMTP_USER", ""),
		SMTPPass:   getEnv("SMTP_PASS", ""),
		SMTPFrom:   getEnv("SMTP_FROM", ""),
		AdminEmail: getEnv("ADMIN_EMAIL", ""),
	}
	return cfg
}

func getEnvAsInt(key string, defaultVal int) int {
	s := getEnv(key, "")
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func InitDB(cfg *Config) *gorm.DB {
	var dialector gorm.Dialector
	if cfg.DBType == "mysql" {
		dialector = mysql.Open(cfg.DBDSN)
	} else {
		dialector = sqlite.Open(cfg.DBDSN)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// 自动迁移
	err = db.AutoMigrate(
		&models.User{},
		&models.License{},
		&models.Activation{},
		&models.OfflineRequest{},
		&models.ApplyRecord{},
		&models.OperationLog{},
	)
	if err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}

	fmt.Println("database initialized:", cfg.DBType)
	return db
}
