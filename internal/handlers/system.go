package handlers

import (
	"net/http"

	"github.com/uuuuupdate/gh-proxy-auth/internal/database"
	"github.com/uuuuupdate/gh-proxy-auth/internal/models"
	"github.com/gin-gonic/gin"
)

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

type InitStatusResponse struct {
	Initialized      bool `json:"initialized"`
	OpenRegistration bool `json:"open_registration"`
}

func (h *SystemHandler) GetInitStatus(c *gin.Context) {
	var count int64
	database.DB.Model(&models.User{}).Count(&count)

	openReg := database.GetSetting("open_registration") == "true"

	c.JSON(http.StatusOK, InitStatusResponse{
		Initialized:      count > 0,
		OpenRegistration: openReg,
	})
}
