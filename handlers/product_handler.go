package handlers

import (
	"net/http"
	"strings"

	"github.com/0x547d/lic/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ProductHandler struct {
	DB *gorm.DB
}

func NewProductHandler(db *gorm.DB) *ProductHandler {
	return &ProductHandler{DB: db}
}

// ListProducts 列出所有产品（管理员）
func (h *ProductHandler) ListProducts(c *gin.Context) {
	var products []models.Product
	h.DB.Order("id ASC").Find(&products)
	c.JSON(http.StatusOK, gin.H{"products": products})
}

// ListProductsPublic 公开产品列表（无需登录，供申请页面使用）
func (h *ProductHandler) ListProductsPublic(c *gin.Context) {
	var products []models.Product
	h.DB.Order("id ASC").Find(&products)
	c.JSON(http.StatusOK, gin.H{"products": products})
}

// CreateProductRequest 创建/更新产品请求
type CreateProductRequest struct {
	ProductKey  string `json:"product_key" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// CreateProduct 新增产品
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.ProductKey = strings.TrimSpace(req.ProductKey)
	req.ProductKey = strings.ToLower(req.ProductKey)

	var existing models.Product
	if err := h.DB.Where("product_key = ?", req.ProductKey).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "product_key already exists"})
		return
	}

	product := models.Product{
		ProductKey:  req.ProductKey,
		Name:        req.Name,
		Description: req.Description,
	}
	h.DB.Create(&product)

	c.JSON(http.StatusCreated, gin.H{"message": "product created", "product": product})
}

// UpdateProduct 修改产品
func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	productKey := c.Param("productKey")

	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var product models.Product
	if err := h.DB.Where("product_key = ?", productKey).First(&product).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	h.DB.Model(&product).Updates(map[string]interface{}{
		"name":        req.Name,
		"description": req.Description,
	})

	c.JSON(http.StatusOK, gin.H{"message": "product updated"})
}

// DeleteProduct 删除产品
func (h *ProductHandler) DeleteProduct(c *gin.Context) {
	productKey := c.Param("productKey")

	result := h.DB.Where("product_key = ?", productKey).Delete(&models.Product{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "product deleted"})
}
