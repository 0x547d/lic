package utils

import (
	"fmt"
	"log"
	"time"

	"github.com/0x547d/lic/config"
	"github.com/0x547d/lic/models"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// CheckResult 授权到期检查结果
type CheckResult struct {
	CheckedCount int      `json:"checked_count"` // 检查了多少条授权
	EmailsSent   int      `json:"emails_sent"`   // 成功发送的邮件数
	Failures     []string `json:"failures"`      // 发送失败的授权码列表（含原因）
}

// StartScheduler 启动定时任务调度器
// 每天上午10:00执行授权到期检查
func StartScheduler(db *gorm.DB, cfg *config.Config) {
	c := cron.New(cron.WithLocation(time.Local))

	// 每天上午10:00执行
	_, err := c.AddFunc("0 10 * * *", func() {
		log.Println("[Scheduler] 开始执行授权到期检查...")
		result := checkExpiringLicenses(db, cfg)
		log.Printf("[Scheduler] 授权到期检查完成: 检查 %d 条，发送 %d 封邮件，失败 %d 条",
			result.CheckedCount, result.EmailsSent, len(result.Failures))
	})
	if err != nil {
		log.Printf("[Scheduler] 添加定时任务失败: %v", err)
		return
	}

	c.Start()
	log.Println("[Scheduler] 定时任务已启动，将在每天上午10:00执行授权到期检查")
}

// RunCheckNow 手动触发授权到期检查，返回详细结果
func RunCheckNow(db *gorm.DB, cfg *config.Config) CheckResult {
	log.Println("[Scheduler] 手动触发授权到期检查...")
	result := checkExpiringLicenses(db, cfg)
	log.Printf("[Scheduler] 手动检查完成: 检查 %d 条，发送 %d 封邮件，失败 %d 条",
		result.CheckedCount, result.EmailsSent, len(result.Failures))
	return result
}

// checkExpiringLicenses 检查即将到期的授权码并发送邮件提醒，返回结果
func checkExpiringLicenses(db *gorm.DB, cfg *config.Config) CheckResult {
	result := CheckResult{}
	now := time.Now()

	// 1. 检查7天内到期的授权（且未发送过7天提醒）
	var weekLicenses []models.License
	db.Where("status = ? AND valid_to > ? AND valid_to <= ? AND notified_7days = ?",
		models.LicenseStatusActive, now, now.Add(7*24*time.Hour), false).
		Find(&weekLicenses)

	result.CheckedCount += len(weekLicenses)
	for _, lic := range weekLicenses {
		if err := sendExpirationWarning(db, cfg, &lic, 7); err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("%s: %v", lic.LicenseKey, err))
		} else {
			result.EmailsSent++
		}
	}

	// 2. 检查1天内到期的授权（且未发送过1天提醒）
	var dayLicenses []models.License
	db.Where("status = ? AND valid_to > ? AND valid_to <= ? AND notified_1day = ?",
		models.LicenseStatusActive, now, now.Add(24*time.Hour), false).
		Find(&dayLicenses)

	result.CheckedCount += len(dayLicenses)
	for _, lic := range dayLicenses {
		if err := sendExpirationWarning(db, cfg, &lic, 1); err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("%s: %v", lic.LicenseKey, err))
		} else {
			result.EmailsSent++
		}
	}

	// 3. 检查已过期但未发送过期通知的授权
	var expiredLicenses []models.License
	db.Where("status = ? AND valid_to <= ? AND notified_expired = ?",
		models.LicenseStatusActive, now, false).
		Find(&expiredLicenses)

	result.CheckedCount += len(expiredLicenses)
	for _, lic := range expiredLicenses {
		if err := sendExpiredNotice(db, cfg, &lic); err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("%s: %v", lic.LicenseKey, err))
		} else {
			result.EmailsSent++
		}
	}

	return result
}

// sendExpirationWarning 发送到期提醒邮件，返回错误
func sendExpirationWarning(db *gorm.DB, cfg *config.Config, lic *models.License, daysLeft int) error {
	// 获取用户信息
	var user models.User
	if err := db.First(&user, lic.UserID).Error; err != nil {
		return fmt.Errorf("获取用户失败: %w", err)
	}

	if user.Email == "" {
		return fmt.Errorf("用户邮箱为空")
	}

	subject, body := BuildExpirationWarning(lic.LicenseKey, daysLeft)
	if err := SendEmail(cfg, user.Email, subject, body); err != nil {
		return fmt.Errorf("发送邮件失败: %w", err)
	}

	// 标记已通知
	if daysLeft == 7 {
		db.Model(lic).Update("notified_7days", true)
	} else if daysLeft == 1 {
		db.Model(lic).Update("notified_1day", true)
	}

	log.Printf("[Scheduler] 已发送到期提醒邮件 to=%s license_key=%s days_left=%d",
		user.Email, lic.LicenseKey, daysLeft)
	return nil
}

// sendExpiredNotice 发送授权已过期通知邮件，返回错误
func sendExpiredNotice(db *gorm.DB, cfg *config.Config, lic *models.License) error {
	// 获取用户信息
	var user models.User
	if err := db.First(&user, lic.UserID).Error; err != nil {
		return fmt.Errorf("获取用户失败: %w", err)
	}

	if user.Email == "" {
		return fmt.Errorf("用户邮箱为空")
	}

	subject, body := BuildExpiredNotice(lic.LicenseKey, lic.ValidTo)
	if err := SendEmail(cfg, user.Email, subject, body); err != nil {
		return fmt.Errorf("发送邮件失败: %w", err)
	}

	// 标记已通知，同时更新授权状态为 expired
	db.Model(lic).Updates(map[string]interface{}{
		"notified_expired": true,
		"status":           models.LicenseStatusExpired,
	})

	log.Printf("[Scheduler] 已发送过期通知邮件 to=%s license_key=%s", user.Email, lic.LicenseKey)
	return nil
}
