package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/uuuuupdate/gh-proxy-auth/internal/database"
	"github.com/uuuuupdate/gh-proxy-auth/internal/models"
	"github.com/gin-gonic/gin"
)

type TokenHandler struct{}

func NewTokenHandler() *TokenHandler {
	return &TokenHandler{}
}

func (h *TokenHandler) List(c *gin.Context) {
	userID := c.GetUint("user_id")
	var tokens []models.Token
	database.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&tokens)
	c.JSON(http.StatusOK, tokens)
}

type CreateTokenRequest struct {
	Name       string `json:"name"`
	ExpireNum  int    `json:"expire_num"`  // 0 means never expire
	ExpireUnit string `json:"expire_unit"` // hour, day
}

func (h *TokenHandler) Create(c *gin.Context) {
	var req CreateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	userID := c.GetUint("user_id")

	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 Token 失败"})
		return
	}
	tokenStr := hex.EncodeToString(tokenBytes)

	token := models.Token{
		UserID: userID,
		Token:  tokenStr,
		Name:   req.Name,
	}

	if req.Name == "" {
		token.Name = "Token-" + tokenStr[:8]
	}

	if req.ExpireNum > 0 {
		var duration time.Duration
		switch req.ExpireUnit {
		case "hour":
			duration = time.Duration(req.ExpireNum) * time.Hour
		case "day":
			duration = time.Duration(req.ExpireNum) * 24 * time.Hour
		default:
			duration = time.Duration(req.ExpireNum) * time.Hour
		}
		expiresAt := time.Now().Add(duration)
		token.ExpiresAt = &expiresAt
	}

	if err := database.DB.Create(&token).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建 Token 失败"})
		return
	}

	c.JSON(http.StatusOK, token)
}

type UpdateTokenRequest struct {
	Name       string `json:"name"`
	ExpireNum  int    `json:"expire_num"`
	ExpireUnit string `json:"expire_unit"`
}

func (h *TokenHandler) Update(c *gin.Context) {
	var req UpdateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	userID := c.GetUint("user_id")
	tokenID := c.Param("id")

	var token models.Token
	if err := database.DB.Where("id = ? AND user_id = ?", tokenID, userID).First(&token).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token 不存在"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}

	if req.ExpireNum > 0 {
		var duration time.Duration
		switch req.ExpireUnit {
		case "hour":
			duration = time.Duration(req.ExpireNum) * time.Hour
		case "day":
			duration = time.Duration(req.ExpireNum) * 24 * time.Hour
		default:
			duration = time.Duration(req.ExpireNum) * time.Hour
		}
		expiresAt := time.Now().Add(duration)
		updates["expires_at"] = &expiresAt
	} else if req.ExpireNum == 0 {
		updates["expires_at"] = nil
	}

	database.DB.Model(&token).Updates(updates)
	database.DB.First(&token, token.ID)
	c.JSON(http.StatusOK, token)
}

func (h *TokenHandler) Delete(c *gin.Context) {
	userID := c.GetUint("user_id")
	tokenID := c.Param("id")

	result := database.DB.Where("id = ? AND user_id = ?", tokenID, userID).Delete(&models.Token{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token 不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Token 已删除"})
}

func (h *TokenHandler) GetLogs(c *gin.Context) {
	userID := c.GetUint("user_id")
	tokenID := c.Param("id")

	var token models.Token
	if err := database.DB.Where("id = ? AND user_id = ?", tokenID, userID).First(&token).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token 不存在"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var logs []models.DownloadLog
	var total int64

	database.DB.Model(&models.DownloadLog{}).Where("token_id = ?", token.ID).Count(&total)
	database.DB.Where("token_id = ?", token.ID).
		Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"data":      logs,
	})
}
