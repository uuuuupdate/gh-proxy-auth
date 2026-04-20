package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/dowork-shanqiu/gh-proxy-auth/internal/database"
	"github.com/dowork-shanqiu/gh-proxy-auth/internal/models"
	"github.com/dowork-shanqiu/gh-proxy-auth/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type TOTPVerifyRequest struct {
	UserID uint   `json:"user_id" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// Check if registration is allowed
	var userCount int64
	database.DB.Model(&models.User{}).Count(&userCount)

	if userCount > 0 {
		openReg := database.GetSetting("open_registration")
		if openReg != "true" {
			c.JSON(http.StatusForbidden, gin.H{"error": "注册未开放"})
			return
		}
	}

	// Check if username exists
	var existing models.User
	if err := database.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}

	user := models.User{
		Username: req.Username,
		Password: string(hash),
		IsAdmin:  userCount == 0, // First user is admin
	}

	if err := database.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败"})
		return
	}

	// Auto-login after registration
	token, err := service.GenerateJWT(user.ID, user.Username, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"is_admin": user.IsAdmin,
		},
	})
}

type LoginResponse struct {
	RequireMFA          bool     `json:"require_mfa,omitempty"`
	MFAType             string   `json:"mfa_type,omitempty"`
	AvailableMFAMethods []string `json:"available_mfa_methods,omitempty"`
	UserID              uint     `json:"user_id,omitempty"`
	Token               string   `json:"token,omitempty"`
	User                gin.H    `json:"user,omitempty"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var user models.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	// Check if MFA is enabled
	var passkeys []models.Passkey
	database.DB.Where("user_id = ?", user.ID).Find(&passkeys)
	hasPasskey := len(passkeys) > 0

	if user.TOTPEnabled || hasPasskey {
		mfaType := user.MFAPriority
		if mfaType == "passkey" && !hasPasskey {
			mfaType = "totp"
		}
		if mfaType == "totp" && !user.TOTPEnabled {
			mfaType = "passkey"
		}

		var available []string
		if hasPasskey {
			available = append(available, "passkey")
		}
		if user.TOTPEnabled {
			available = append(available, "totp")
		}

		resp := LoginResponse{
			RequireMFA:          true,
			MFAType:             mfaType,
			AvailableMFAMethods: available,
			UserID:              user.ID,
		}

		c.JSON(http.StatusOK, resp)
		return
	}

	token, err := service.GenerateJWT(user.ID, user.Username, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User: gin.H{
			"id":       user.ID,
			"username": user.Username,
			"is_admin": user.IsAdmin,
		},
	})
}

func (h *AuthHandler) VerifyTOTP(c *gin.Context) {
	var req TOTPVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if !user.TOTPEnabled || user.TOTPSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "TOTP 未启用"})
		return
	}

	if !service.ValidateTOTP(user.TOTPSecret, req.Code) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "验证码错误"})
		return
	}

	token, err := service.GenerateJWT(user.ID, user.Username, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"is_admin": user.IsAdmin,
		},
	})
}

func (h *AuthHandler) BeginPasskeyLogin(c *gin.Context) {
	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	webauthnUser, err := service.NewWebAuthnUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户凭证失败"})
		return
	}

	options, session, err := service.WebAuthn.BeginLogin(webauthnUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "开始认证失败: " + err.Error()})
		return
	}

	if err := service.StoreSessionData(user.ID, session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存会话失败"})
		return
	}

	c.JSON(http.StatusOK, options)
}

func (h *AuthHandler) FinishPasskeyLogin(c *gin.Context) {
	var req struct {
		UserID uint `json:"user_id"`
	}

	if rawID := c.Query("user_id"); rawID != "" {
		if id, err := strconv.Atoi(rawID); err == nil && id > 0 {
			req.UserID = uint(id)
		}
	}

	if req.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	sessionJSON, err := service.GetSessionData(user.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会话已过期"})
		return
	}

	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析会话失败"})
		return
	}

	// Parse the credential response first to get the flags
	parsedResponse, err := protocol.ParseCredentialRequestResponse(c.Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "解析认证响应失败: " + err.Error()})
		return
	}

	// Get the BackupEligible flag from the parsed response
	beFlag := parsedResponse.Response.AuthenticatorData.Flags.HasBackupEligible()
	bsFlag := parsedResponse.Response.AuthenticatorData.Flags.HasBackupState()

	// Update the passkey's BackupEligible flag in DB before validation
	// This handles legacy passkeys that were registered before the flag was stored
	credIDBase64 := bufferToBase64RawURL(parsedResponse.RawID)
	database.DB.Model(&models.Passkey{}).Where("user_id = ? AND credential_id = ?", user.ID, credIDBase64).
		Updates(map[string]interface{}{
			"backup_eligible": beFlag,
			"backup_state":    bsFlag,
		})

	// Now create the WebAuthnUser with updated flags
	webauthnUser, err := service.NewWebAuthnUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户凭证失败"})
		return
	}

	credential, err := service.WebAuthn.ValidateLogin(webauthnUser, session, parsedResponse)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "认证失败: " + err.Error()})
		return
	}

	service.ClearSessionData(user.ID)

	// Update sign count and flags after successful login
	database.DB.Model(&models.Passkey{}).Where("user_id = ? AND credential_id = ?", user.ID, credIDBase64).
		Updates(map[string]interface{}{
			"sign_count":      credential.Authenticator.SignCount,
			"backup_eligible": credential.Flags.BackupEligible,
			"backup_state":    credential.Flags.BackupState,
		})

	token, err := service.GenerateJWT(user.ID, user.Username, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"is_admin": user.IsAdmin,
		},
	})
}

func bufferToBase64RawURL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
