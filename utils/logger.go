package utils

import (
	"log"
	"time"

	"license-server/models"
	"gorm.io/gorm"
)

// LogOperation 记录管理员操作日志
// action 建议取值：create / revoke / extend / activate / deactivate / approve / reject / register
func LogOperation(db *gorm.DB, adminID uint, adminName, action, target, detail, ip string) {
	logEntry := models.OperationLog{
		AdminID:   adminID,
		AdminName: adminName,
		Action:    action,
		Target:    target,
		Detail:    detail,
		IP:        ip,
		CreatedAt: time.Now(),
	}
	if err := db.Create(&logEntry).Error; err != nil {
		log.Printf("[LogOperation] failed to write log: %v", err)
	}
}
