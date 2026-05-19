package utils

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"license-server/config"
	"license-server/models"
)

// SendEmail 发送邮件（使用 SMTP）
func SendEmail(cfg *config.Config, to, subject, bodyHTML string) error {
	if cfg.SMTPHost == "" || cfg.SMTPUser == "" {
		// SMTP 未配置，仅打印日志
		fmt.Printf("[Email] to=%s subject=%s body=%s\n", to, subject, bodyHTML)
		return nil
	}

	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)

	// 构建邮件内容
	from := cfg.SMTPFrom
	if from == "" {
		from = cfg.SMTPUser
	}
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, to, subject, bodyHTML,
	))

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	var err error
	if cfg.SMTPPort == 465 {
		// SSL 连接
		err = sendWithSSL(addr, auth, from, []string{to}, msg)
	} else {
		// STARTTLS
		err = smtp.SendMail(addr, auth, from, []string{to}, msg)
	}
	return err
}

func sendWithSSL(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	// 端口 465 需要先在 TLS 连接上通信（SMTPS）
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		MinVersion: tls.VersionTLS12,
		InsecureSkipVerify: false,
	})
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, strings.Split(addr, ":")[0])
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	if err = c.Auth(auth); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err = c.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return fmt.Errorf("rcpt to: %w", err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	err = w.Close()
	if err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return c.Quit()
}

// 邮件模板

func BuildApplyNotifyAdmin(record *models.ApplyRecord) (string, string) {
	subject := fmt.Sprintf("[授权申请] 新申请：%s", record.ApplicantName)
	body := fmt.Sprintf(`
		<html><body>
		<h3>新的授权码申请</h3>
		<p><strong>申请人：</strong>%s</p>
		<p><strong>邮箱：</strong>%s</p>
		<p><strong>电话：</strong>%s</p>
		<p><strong>产品类型：</strong>%s</p>
		<p><strong>授权时长：</strong>%d 个月</p>
		<p><strong>最大激活数：</strong>%d</p>
		<p><strong>说明：</strong>%s</p>
		<p><small>申请时间：%s</small></p>
		<hr><p>请登录管理后台处理此申请。</p>
		</body></html>`,
		record.ApplicantName, record.Email, record.Phone,
		record.ProductType, record.DurationMonths, record.MaxActivations,
		record.Description, record.CreatedAt.Format("2006-01-02 15:04:05"),
	)
	return subject, body
}

func BuildApplyApproved(records *models.ApplyRecord, licenseKey string) (string, string) {
	subject := "【授权申请已通过】"
	body := fmt.Sprintf(`
		<html><body>
		<h3>您的授权申请已通过审核</h3>
		<p>感谢您的申请，以下是您的授权信息：</p>
		<div style="background:#f5f7fa;padding:16px;border-radius:6px;margin:16px 0;">
			<p><strong>授权码：</strong><code>%s</code></p>
			<p><strong>授权时长：</strong>%d 个月</p>
			<p><strong>最大激活数：</strong>%d 台设备</p>
		</div>
		<p>请在客户端中输入授权码完成激活。</p>
		<p style="color:#909399;font-size:12px;">如有疑问，请联系客服邮箱</p>
		</body></html>`,
		licenseKey, records.DurationMonths, records.MaxActivations,
	)
	return subject, body
}

func BuildApplyRejected(records *models.ApplyRecord, adminNote string) (string, string) {
	subject := "【授权申请未通过】"
	body := fmt.Sprintf(`
		<html><body>
		<h3>您的授权申请未通过审核</h3>
		<p>很抱歉，您的授权申请未能通过审核。</p>
		%s
		<p>如有疑问，请联系客服邮箱。</p>
		</body></html>`,
		func() string {
			if adminNote != "" {
				return "<p><strong>审核备注：</strong>" + adminNote + "</p>"
			}
			return ""
		}(),
	)
	return subject, body
}

// BuildExpirationWarning 授权即将到期提醒
func BuildExpirationWarning(licenseKey string, daysLeft int) (string, string) {
	subject := fmt.Sprintf("【授权即将到期】还剩 %d 天", daysLeft)
	body := fmt.Sprintf(`
		<html><body>
		<h3>您的授权即将到期</h3>
		<div style="background:#fff3cd;padding:16px;border-radius:6px;margin:16px 0;border-left:4px solid #ffc107;">
			<p><strong>授权码：</strong><code>%s</code></p>
			<p><strong>剩余有效期：</strong>%d 天</p>
		</div>
		<p>请及时联系管理员续约，以免影响正常使用。</p>
		<p style="color:#909399;font-size:12px;">此为自动发送邮件，请勿直接回复</p>
		</body></html>`,
		licenseKey, daysLeft,
	)
	return subject, body
}

// BuildExpiredNotice 授权已过期通知
func BuildExpiredNotice(licenseKey string, expiredAt time.Time) (string, string) {
	subject := "【授权已过期】请尽快续约"
	body := fmt.Sprintf(`
		<html><body>
		<h3>您的授权已过期</h3>
		<div style="background:#f8d7da;padding:16px;border-radius:6px;margin:16px 0;border-left:4px solid #dc3545;">
			<p><strong>授权码：</strong><code>%s</code></p>
			<p><strong>过期时间：</strong>%s</p>
		</div>
		<p>授权已失效，请尽快联系管理员续约以恢复使用。</p>
		<p style="color:#909399;font-size:12px;">此为自动发送邮件，请勿直接回复</p>
		</body></html>`,
		licenseKey, expiredAt.Format("2006-01-02"),
	)
	return subject, body
}
