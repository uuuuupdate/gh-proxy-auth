package handlers

import (
"net/http"
"strconv"
"time"

"github.com/dowork-shanqiu/gh-proxy-auth/internal/database"
"github.com/dowork-shanqiu/gh-proxy-auth/internal/models"
"github.com/gin-gonic/gin"
)

type AdminHandler struct{}

func NewAdminHandler() *AdminHandler {
return &AdminHandler{}
}

func (h *AdminHandler) GetSettings(c *gin.Context) {
openReg := database.GetSetting("open_registration") == "true"
globalLimit, _ := strconv.ParseInt(database.GetSetting("global_speed_limit"), 10, 64)
c.JSON(http.StatusOK, gin.H{
"open_registration":  openReg,
"global_speed_limit": globalLimit,
})
}

func (h *AdminHandler) UpdateSettings(c *gin.Context) {
var req struct {
OpenRegistration bool  `json:"open_registration"`
GlobalSpeedLimit int64 `json:"global_speed_limit"`
}
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
return
}

val := "false"
if req.OpenRegistration {
val = "true"
}
if err := database.SetSetting("open_registration", val); err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "更新设置失败"})
return
}

limitStr := strconv.FormatInt(req.GlobalSpeedLimit, 10)
if err := database.SetSetting("global_speed_limit", limitStr); err != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "更新设置失败"})
return
}
// Apply global limit immediately
SetGlobalLimit(req.GlobalSpeedLimit)

c.JSON(http.StatusOK, gin.H{"message": "设置已更新"})
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
if page < 1 {
page = 1
}
if pageSize < 1 || pageSize > 100 {
pageSize = 20
}

var users []models.User
var total int64

database.DB.Model(&models.User{}).Count(&total)
database.DB.Order("created_at desc").
Offset((page - 1) * pageSize).
Limit(pageSize).
Find(&users)

c.JSON(http.StatusOK, gin.H{
"total":     total,
"page":      page,
"page_size": pageSize,
"data":      users,
})
}

func (h *AdminHandler) UpdateUserSpeedLimit(c *gin.Context) {
userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
if err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户 ID"})
return
}

var req struct {
SpeedLimit int64 `json:"speed_limit"`
}
if err := c.ShouldBindJSON(&req); err != nil {
c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
return
}
if req.SpeedLimit < 0 {
req.SpeedLimit = 0
}

result := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("speed_limit", req.SpeedLimit)
if result.Error != nil {
c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
return
}
if result.RowsAffected == 0 {
c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
return
}

c.JSON(http.StatusOK, gin.H{"message": "限速已更新"})
}

func (h *AdminHandler) GetDownloadLogs(c *gin.Context) {
page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
if page < 1 {
page = 1
}
if pageSize < 1 || pageSize > 100 {
pageSize = 20
}

var total int64
database.DB.Model(&models.DownloadLog{}).Count(&total)

type logScan struct {
ID        uint
CreatedAt time.Time
URL       string
IP        string
Username  string
TokenName string
}

var rows []logScan
database.DB.Model(&models.DownloadLog{}).
Select("download_logs.id, download_logs.created_at, download_logs.url, download_logs.ip, users.username, tokens.name as token_name").
Joins("LEFT JOIN users ON users.id = download_logs.user_id").
Joins("LEFT JOIN tokens ON tokens.id = download_logs.token_id").
Order("download_logs.created_at desc").
Offset((page - 1) * pageSize).
Limit(pageSize).
Scan(&rows)

type LogItem struct {
ID        uint   `json:"id"`
CreatedAt string `json:"created_at"`
Username  string `json:"username"`
TokenName string `json:"token_name"`
URL       string `json:"url"`
IP        string `json:"ip"`
}

items := make([]LogItem, 0, len(rows))
for _, r := range rows {
items = append(items, LogItem{
ID:        r.ID,
CreatedAt: r.CreatedAt.Format("2006-01-02 15:04:05"),
Username:  r.Username,
TokenName: r.TokenName,
URL:       r.URL,
IP:        r.IP,
})
}

c.JSON(http.StatusOK, gin.H{
"total":     total,
"page":      page,
"page_size": pageSize,
"data":      items,
})
}
