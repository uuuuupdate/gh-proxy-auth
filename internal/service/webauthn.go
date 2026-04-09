package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/dowork-shanqiu/gh-proxy-auth/internal/config"
	"github.com/dowork-shanqiu/gh-proxy-auth/internal/database"
	"github.com/dowork-shanqiu/gh-proxy-auth/internal/models"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

var WebAuthn *webauthn.WebAuthn

func InitWebAuthn() error {
	domain := config.C.Domain
	u, err := url.Parse(domain)
	if err != nil {
		return fmt.Errorf("failed to parse domain: %w", err)
	}

	rpID := u.Hostname()
	rpOrigin := strings.TrimRight(domain, "/")

	wconfig := &webauthn.Config{
		RPDisplayName: "GH Proxy Auth",
		RPID:          rpID,
		RPOrigins:     []string{rpOrigin},
	}

	WebAuthn, err = webauthn.New(wconfig)
	if err != nil {
		return fmt.Errorf("failed to create webauthn: %w", err)
	}
	return nil
}

type WebAuthnUser struct {
	user    models.User
	passkeys []models.Passkey
}

func NewWebAuthnUser(user models.User) (*WebAuthnUser, error) {
	var passkeys []models.Passkey
	if err := database.DB.Where("user_id = ?", user.ID).Find(&passkeys).Error; err != nil {
		return nil, err
	}
	return &WebAuthnUser{user: user, passkeys: passkeys}, nil
}

func (u *WebAuthnUser) WebAuthnID() []byte {
	return []byte(fmt.Sprintf("%d", u.user.ID))
}

func (u *WebAuthnUser) WebAuthnName() string {
	return u.user.Username
}

func (u *WebAuthnUser) WebAuthnDisplayName() string {
	return u.user.Username
}

func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, 0, len(u.passkeys))
	for _, p := range u.passkeys {
		credID, _ := base64.RawURLEncoding.DecodeString(p.CredentialID)
		var transports []protocol.AuthenticatorTransport
		if p.Transport != "" {
			parts := strings.Split(p.Transport, ",")
			for _, t := range parts {
				transports = append(transports, protocol.AuthenticatorTransport(t))
			}
		}
		creds = append(creds, webauthn.Credential{
			ID:              credID,
			PublicKey:       p.PublicKey,
			AttestationType: p.AttestationType,
			Transport:       transports,
			Authenticator: webauthn.Authenticator{
				SignCount: p.SignCount,
			},
		})
	}
	return creds
}

func (u *WebAuthnUser) WebAuthnIcon() string {
	return ""
}

func StoreSessionData(userID uint, sessionData interface{}) error {
	data, err := json.Marshal(sessionData)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("webauthn_session_%d", userID)
	return database.SetSetting(key, string(data))
}

func GetSessionData(userID uint) (string, error) {
	key := fmt.Sprintf("webauthn_session_%d", userID)
	val := database.GetSetting(key)
	if val == "" {
		return "", fmt.Errorf("no session data found")
	}
	return val, nil
}

func ClearSessionData(userID uint) {
	key := fmt.Sprintf("webauthn_session_%d", userID)
	database.DB.Unscoped().Where("key = ?", key).Delete(&models.Setting{})
}
