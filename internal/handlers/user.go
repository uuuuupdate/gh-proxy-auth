package handlers

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/dowork-shanqiu/gh-proxy-auth/internal/database"
	"github.com/dowork-shanqiu/gh-proxy-auth/internal/models"
	"github.com/dowork-shanqiu/gh-proxy-auth/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"
)

type UserHandler struct{}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetUint("user_id")
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	var passkeyCount int64
	database.DB.Model(&models.Passkey{}).Where("user_id = ?", userID).Count(&passkeyCount)

	c.JSON(http.StatusOK, gin.H{
		"id":            user.ID,
		"username":      user.Username,
		"is_admin":      user.IsAdmin,
		"totp_enabled":  user.TOTPEnabled,
		"mfa_priority":  user.MFAPriority,
		"passkey_count": passkeyCount,
	})
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	userID := c.GetUint("user_id")
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码错误"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误"})
		return
	}

	database.DB.Model(&user).Update("password", string(hash))
	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

func (h *UserHandler) SetupTOTP(c *gin.Context) {
	userID := c.GetUint("user_id")
	username := c.GetString("username")

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	issuer := "GH-Proxy-Auth"
	secret, qrBase64, err := service.GenerateTOTP(username, issuer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 TOTP 失败"})
		return
	}

	// Store secret temporarily (not yet enabled)
	database.DB.Model(&user).Update("totp_secret", secret)

	c.JSON(http.StatusOK, gin.H{
		"secret":   secret,
		"qr_image": qrBase64,
	})
}

func (h *UserHandler) EnableTOTP(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	userID := c.GetUint("user_id")
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	if user.TOTPSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先设置 TOTP"})
		return
	}

	if !service.ValidateTOTP(user.TOTPSecret, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误，请重试"})
		return
	}

	database.DB.Model(&user).Update("totp_enabled", true)
	c.JSON(http.StatusOK, gin.H{"message": "TOTP 已启用"})
}

func (h *UserHandler) DisableTOTP(c *gin.Context) {
	userID := c.GetUint("user_id")
	database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"totp_enabled": false,
		"totp_secret":  "",
	})
	c.JSON(http.StatusOK, gin.H{"message": "TOTP 已关闭"})
}

func (h *UserHandler) SetMFAPriority(c *gin.Context) {
	var req struct {
		Priority string `json:"priority" binding:"required,oneof=passkey totp"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}

	userID := c.GetUint("user_id")
	database.DB.Model(&models.User{}).Where("id = ?", userID).Update("mfa_priority", req.Priority)
	c.JSON(http.StatusOK, gin.H{"message": "MFA 优先级已更新"})
}

func (h *UserHandler) ListPasskeys(c *gin.Context) {
	userID := c.GetUint("user_id")
	var passkeys []models.Passkey
	database.DB.Where("user_id = ?", userID).Find(&passkeys)
	c.JSON(http.StatusOK, passkeys)
}

func (h *UserHandler) BeginRegisterPasskey(c *gin.Context) {
	userID := c.GetUint("user_id")
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	webauthnUser, err := service.NewWebAuthnUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户凭证失败"})
		return
	}

	options, session, err := service.WebAuthn.BeginRegistration(webauthnUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "开始注册失败: " + err.Error()})
		return
	}

	if err := service.StoreSessionData(user.ID, session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存会话失败"})
		return
	}

	c.JSON(http.StatusOK, options)
}

func (h *UserHandler) FinishRegisterPasskey(c *gin.Context) {
	var nameReq struct {
		Name string `json:"name"`
	}
	passkeyName := c.Query("name")
	if passkeyName == "" {
		_ = c.ShouldBindJSON(&nameReq)
		passkeyName = nameReq.Name
	}
	if passkeyName == "" {
		passkeyName = "My Passkey"
	}

	userID := c.GetUint("user_id")
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	webauthnUser, err := service.NewWebAuthnUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户凭证失败"})
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

	credential, err := service.WebAuthn.FinishRegistration(webauthnUser, session, c.Request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "注册失败: " + err.Error()})
		return
	}

	service.ClearSessionData(user.ID)

	// Build transport string
	transports := ""
	for i, t := range credential.Transport {
		if i > 0 {
			transports += ","
		}
		transports += string(t)
	}

	passkey := models.Passkey{
		UserID:          user.ID,
		Name:            passkeyName,
		CredentialID:    base64.RawURLEncoding.EncodeToString(credential.ID),
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          base64.RawURLEncoding.EncodeToString(credential.Authenticator.AAGUID),
		SignCount:       credential.Authenticator.SignCount,
		Transport:       transports,
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
	}

	if err := database.DB.Create(&passkey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存凭证失败"})
		return
	}

	// Refresh JWT to include updated state
	token, _ := service.GenerateJWT(user.ID, user.Username, user.IsAdmin)

	c.JSON(http.StatusOK, gin.H{
		"message": "Passkey 注册成功",
		"passkey": passkey,
		"token":   token,
	})
}

func (h *UserHandler) DeletePasskey(c *gin.Context) {
	userID := c.GetUint("user_id")
	passkeyID := c.Param("id")

	result := database.DB.Where("id = ? AND user_id = ?", passkeyID, userID).Delete(&models.Passkey{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Passkey 不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Passkey 已删除"})
}
