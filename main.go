package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/0x547d/lic/config"
	"github.com/0x547d/lic/handlers"
	"github.com/0x547d/lic/middleware"
	"github.com/0x547d/lic/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// 初始化数据库
	db := config.InitDB(cfg)

	// 启动定时任务（每天上午10:00检查授权到期）
	utils.StartScheduler(db, cfg)

	// 初始化 RSA 密钥（用于离线激活签名）
	if err := utils.InitRSA("", ""); err != nil {
		log.Fatalf("failed to init RSA: %v", err)
	}
	utils.JWTSecret = []byte(cfg.JWTSecret)

	// 设置 gin 模式
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 加载 HTML 模板
	r.LoadHTMLGlob("templates/*")
	// 静态文件服务
	r.Static("/static", "./static")

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 创建 handler
	authHandler := handlers.NewAuthHandler(db)
	licenseHandler := handlers.NewLicenseHandler(db)
	webHandler := handlers.NewWebHandler(db, cfg)

	// ===== Web 页面路由 =====
	r.GET("/", webHandler.ApplyPage)                         // 客户自助申请页面
	r.GET("/admin/login", webHandler.AdminLoginPage)         // 管理后台登录页
	r.GET("/admin/dashboard", webHandler.AdminDashboardPage) // 管理后台主页（前端鉴权）

	// ===== 公开 API =====
	public := r.Group("/api/v1")
	{
		public.POST("/register", authHandler.Register)            // 注册用户（自动生成授权码）
		public.POST("/login", authHandler.Login)                  // 帐密登录获取 Token
		public.POST("/verify", authHandler.VerifyToken)           // 验证授权状态
		public.POST("/offline/verify", authHandler.OfflineVerify) // 客户端验证离线响应文件
		public.POST("/offline/request/gen", authHandler.OfflineRequestGen)
		public.GET("/offline/request/:token/download", authHandler.OfflineRequestDownload)
		public.POST("/offline/activate/:token", authHandler.OfflineActivate)
		public.POST("/apply", webHandler.HandleApply)        // 客户提交授权申请
		public.POST("/admin/web-login", webHandler.WebLogin) // 管理后台登录（返回 Token）
	}

	// ===== 需要 JWT 认证的 API =====
	protected := r.Group("/api/v1")
	protected.Use(middleware.JWTAuth())
	{
		protected.POST("/activate", authHandler.ActivateOnline)
		protected.GET("/license/me", licenseHandler.GetLicense)
		protected.GET("/licenses", licenseHandler.ListLicenses)
	}

	// ===== 管理员 API（带 JWT 鉴权）=====
	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.JWTAuth())
	{
		admin.GET("/check-expiring", func(c *gin.Context) {
			utils.RunCheckNow(db, cfg)
			c.JSON(http.StatusOK, gin.H{"message": "expiration check triggered"})
		})
		admin.POST("/license", licenseHandler.CreateLicense)
		admin.GET("/licenses", licenseHandler.ListLicenses)
		admin.GET("/license/:licenseKey", licenseHandler.GetLicense)
		admin.PUT("/license/:licenseKey/revoke", licenseHandler.RevokeLicense)
		admin.PUT("/license/:licenseKey/extend", licenseHandler.ExtendLicense)
		admin.GET("/license/:licenseKey/activations", licenseHandler.ListActivations)
		admin.PUT("/license/:licenseKey/deactivate/:deviceFP", licenseHandler.DeactivateDevice)
		admin.GET("/logs", webHandler.ListOperationLogs)
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 启动服务
	addr := cfg.HTTPAddr
	fmt.Printf("🚀 License Server starting on %s\n", addr)
	fmt.Printf("   Web:       http://localhost%s/\n", addr)
	fmt.Printf("   Admin:     http://localhost%s/admin/login\n", addr)
	fmt.Printf("   API Login: POST http://localhost%s/api/v1/login\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
