package utils

import (
	"log"
	"time"

	"github.com/0x547d/lic/config"
	"github.com/0x547d/lic/models"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// StartScheduler 启动定时任务调度器
// 每天上午10:00执行授权到期检查
func StartScheduler(db *gorm.DB, cfg *config.Config) {
	c := cron.New(cron.WithLocation(time.Local))

	// 每天上午10:00执行
	_, err := c.AddFunc("0 10 * * *", func() {
		log.Println("[Scheduler] 开始执行授权到期检查...")
		checkExpiringLicenses(db, cfg)
		log.Println("[Scheduler] 授权到期检查完成")
	})
	if err != nil {
		log.Printf("[Scheduler] 添加定时任务失败: %v", err)
		return
	}

	c.Start()
	log.Println("[Scheduler] 定时任务已启动，将在每天上午10:00执行授权到期检查")
}

// RunCheckNow 手动触发授权到期检查（用于测试，不启动 cron）
func RunCheckNow(db *gorm.DB, cfg *config.Config) {
	log.Println("[Scheduler] 手动触发授权到期检查...")
	checkExpiringLicenses(db, cfg)
	log.Println("[Scheduler] 手动检查完成")
}

// checkExpiringLicenses 检查即将到期的授权码并发送邮件提醒
func checkExpiringLicenses(db *gorm.DB, cfg *config.Config) {
	now := time.Now()

	// 1. 检查7天内到期的授权（且未发送过7天提醒）
	var weekLicenses []models.License
	db.Where("status = ? AND valid_to > ? AND valid_to <= ? AND notified_7days = ?",
		models.LicenseStatusActive, now, now.Add(7*24*time.Hour), false).
		Find(&weekLicenses)

	for _, lic := range weekLicenses {
		sendExpirationWarning(db, cfg, &lic, 7)
	}

	// 2. 检查1天内到期的授权（且未发送过1天提醒）
	var dayLicenses []models.License
	db.Where("status = ? AND valid_to > ? AND valid_to <= ? AND notified_1day = ?",
		models.LicenseStatusActive, now, now.Add(24*time.Hour), false).
		Find(&dayLicenses)

	for _, lic := range dayLicenses {
		sendExpirationWarning(db, cfg, &lic, 1)
	}

	// 3. 检查已过期但未发送过期通知的授权
	var expiredLicenses []models.License
	db.Where("status = ? AND valid_to <= ? AND notified_expired = ?",
		models.LicenseStatusActive, now, false).
		Find(&expiredLicenses)

	for _, lic := range expiredLicenses {
		sendExpiredNotice(db, cfg, &lic)
	}
}

// sendExpirationWarning 发送到期提醒邮件
func sendExpirationWarning(db *gorm.DB, cfg *config.Config, lic *models.License, daysLeft int) {
	// 获取用户信息
	var user models.User
	if err := db.First(&user, lic.UserID).Error; err != nil {
		log.Printf("[Scheduler] 获取用户失败 license_key=%s: %v", lic.LicenseKey, err)
		return
	}

	if user.Email == "" {
		log.Printf("[Scheduler] 用户邮箱为空，跳过提醒 license_key=%s", lic.LicenseKey)
		return
	}

	subject, body := BuildExpirationWarning(lic.LicenseKey, daysLeft)
	if err := SendEmail(cfg, user.Email, subject, body); err != nil {
		log.Printf("[Scheduler] 发送到期提醒邮件失败 to=%s: %v", user.Email, err)
		return
	}

	// 标记已通知
	if daysLeft == 7 {
		db.Model(lic).Update("notified_7days", true)
	} else if daysLeft == 1 {
		db.Model(lic).Update("notified_1day", true)
	}

	log.Printf("[Scheduler] 已发送到期提醒邮件 to=%s license_key=%s days_left=%d",
		user.Email, lic.LicenseKey, daysLeft)
}

// sendExpiredNotice 发送授权已过期通知邮件
func sendExpiredNotice(db *gorm.DB, cfg *config.Config, lic *models.License) {
	// 获取用户信息
	var user models.User
	if err := db.First(&user, lic.UserID).Error; err != nil {
		log.Printf("[Scheduler] 获取用户失败 license_key=%s: %v", lic.LicenseKey, err)
		return
	}

	if user.Email == "" {
		log.Printf("[Scheduler] 用户邮箱为空，跳过过期通知 license_key=%s", lic.LicenseKey)
		return
	}

	subject, body := BuildExpiredNotice(lic.LicenseKey, lic.ValidTo)
	if err := SendEmail(cfg, user.Email, subject, body); err != nil {
		log.Printf("[Scheduler] 发送过期通知邮件失败 to=%s: %v", user.Email, err)
		return
	}

	// 标记已通知，同时更新授权状态为 expired
	db.Model(lic).Updates(map[string]interface{}{
		"notified_expired": true,
		"status":           models.LicenseStatusExpired,
	})

	log.Printf("[Scheduler] 已发送过期通知邮件 to=%s license_key=%s", user.Email, lic.LicenseKey)
}
