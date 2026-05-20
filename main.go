package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/0x547d/lic/config"
	"github.com/0x547d/lic/handlers"
	"github.com/0x547d/lic/middleware"
	"github.com/0x547d/lic/models"
	"github.com/0x547d/lic/utils"
)

func main() {
	cfg := config.Load()

	// 初始化数据库
	db := config.InitDB(cfg)

	// Seed 默认产品（如不存在则创建）
	seedDefaultProducts(db)

	// 迁移旧 product_key 数据到 product_keys
	migrateProductKeys(db)

	// 启动定时任务（每天上午10:00检查授权到期）
	utils.StartScheduler(db, cfg)

	// 初始化 RSA 密钥（用于离线激活签名）
	// 优先级：环境变量 > 默认路径 > 自动生成临时密钥
	rsaPrivateKeyPath := os.Getenv("RSA_PRIVATE_KEY_PATH")
	rsaPublicKeyPath := os.Getenv("RSA_PUBLIC_KEY_PATH")

	// 如果环境变量未设置，使用配置中的路径
	if rsaPrivateKeyPath == "" {
		rsaPrivateKeyPath = cfg.RSAPrivateKeyPath
	}
	if rsaPublicKeyPath == "" {
		rsaPublicKeyPath = cfg.RSAPublicKeyPath
	}

	if err := utils.InitRSA(rsaPrivateKeyPath, rsaPublicKeyPath); err != nil {
		log.Printf("Warning: failed to load RSA from file, using temporary key: %v", err)
		// 注意：如果加载文件失败，会继续使用自动生成的临时密钥
	}
	utils.JWTSecret = []byte(cfg.JWTSecret)

	// 设置 gin 模式
	gin.SetMode(gin.ReleaseMode)
	gin.DisableConsoleColor()
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
	productHandler := handlers.NewProductHandler(db)

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
		public.POST("/apply", webHandler.HandleApply)              // 客户提交授权申请
		public.GET("/products", productHandler.ListProductsPublic) // 公开产品列表
		public.POST("/admin/web-login", webHandler.WebLogin)       // 管理后台登录（返回 Token）
	}

	// ===== 需要 JWT 认证的 API =====
	protected := r.Group("/api/v1")
	protected.Use(middleware.JWTAuth())
	{
		protected.POST("/activate", authHandler.ActivateOnline)
		protected.GET("/license/me", licenseHandler.GetLicense)
		protected.GET("/licenses", licenseHandler.ListLicenses)
		protected.GET("/license/:licenseKey/offline-auth-file", authHandler.DownloadOfflineAuthFile)
	}

	// ===== 管理员 API（带 JWT 鉴权）=====
	admin := r.Group("/api/v1/admin")
	admin.Use(middleware.JWTAuth())
	{
		admin.GET("/check-expiring", func(c *gin.Context) {
			result := utils.RunCheckNow(db, cfg)
			c.JSON(http.StatusOK, gin.H{
				"message":       "expiration check completed",
				"checked_count": result.CheckedCount,
				"emails_sent":   result.EmailsSent,
				"failures":      result.Failures,
			})
		})
		admin.POST("/license", licenseHandler.CreateLicense)
		admin.GET("/licenses", licenseHandler.ListLicenses)
		admin.GET("/license/:licenseKey", licenseHandler.GetLicense)
		admin.PUT("/license/:licenseKey/revoke", licenseHandler.RevokeLicense)
		admin.PUT("/license/:licenseKey/extend", licenseHandler.ExtendLicense)
		admin.GET("/license/:licenseKey/activations", licenseHandler.ListActivations)
		admin.PUT("/license/:licenseKey/deactivate/:deviceFP", licenseHandler.DeactivateDevice)
		admin.PUT("/license/:licenseKey/disable", licenseHandler.DisableLicense)
		admin.PUT("/license/:licenseKey/enable", licenseHandler.EnableLicense)
		admin.GET("/logs", webHandler.ListOperationLogs)
		// 产品管理
		admin.GET("/products", productHandler.ListProducts)
		admin.POST("/products", productHandler.CreateProduct)
		admin.PUT("/products/:productKey", productHandler.UpdateProduct)
		admin.DELETE("/products/:productKey", productHandler.DeleteProduct)
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// 启动服务
	addr := cfg.HTTPAddr
	log.Printf("🚀 License Server starting on %s", addr)
	log.Printf("   Web:       http://localhost%s/\n", addr)
	log.Printf("   Admin:     http://localhost%s/admin/login\n", addr)
	log.Printf("   API Login: POST http://localhost%s/api/v1/login\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

// seedDefaultProducts 初始化默认产品数据
func seedDefaultProducts(db *gorm.DB) {
	defaults := []models.Product{
		{ProductKey: "standard", Name: "标准版", Description: "基础功能版本"},
		{ProductKey: "pro", Name: "专业版", Description: "专业功能版本"},
		{ProductKey: "enterprise", Name: "企业版", Description: "企业级功能版本"},
		{ProductKey: "trial", Name: "试用版", Description: "免费试用版本"},
	}
	for _, p := range defaults {
		var existing models.Product
		if err := db.Where("product_key = ?", p.ProductKey).First(&existing).Error; err != nil {
			db.Create(&p)
		}
	}
}

// migrateProductKeys 将旧 product_key 列数据迁移到 product_keys（仅在有旧列时执行）
func migrateProductKeys(db *gorm.DB) {
	migrate := func(table string) {
		rows, err := db.Raw("SELECT id, product_key FROM "+table+" WHERE product_keys IS NULL OR product_keys = ?", "").Rows()
		if err != nil {
			return
		}
		defer rows.Close()
		type rec struct {
			ID  uint
			Key string
		}
		var batch []rec
		for rows.Next() {
			var r rec
			rows.Scan(&r.ID, &r.Key)
			if r.Key != "" {
				batch = append(batch, r)
			}
		}
		for _, r := range batch {
			db.Table(table).Where("id = ?", r.ID).Update("product_keys", models.JSONSlice{r.Key})
		}
	}
	migrate("licenses")
	migrate("apply_records")
}
