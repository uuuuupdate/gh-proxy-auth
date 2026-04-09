package handlers

import (
"io"
"log"
"net"
"net/http"
"net/url"
"regexp"
"strings"
"time"

"github.com/dowork-shanqiu/gh-proxy-auth/internal/config"
"github.com/dowork-shanqiu/gh-proxy-auth/internal/database"
"github.com/dowork-shanqiu/gh-proxy-auth/internal/models"
"github.com/gin-gonic/gin"
"gorm.io/gorm"
)

const (
// proxyBufSize is the chunk size used for throttled streaming.
proxyBufSize = 32 * 1024 // 32KB
// proxyCopyBuf is the buffer size for unthrottled streaming.
proxyCopyBuf = 256 * 1024 // 256KB
)

var (
exp1 = regexp.MustCompile(`^(?:https?://)?github\.com/(.+?)/(.+?)/(?:releases|archive)/.*$`)
exp2 = regexp.MustCompile(`^(?:https?://)?github\.com/(.+?)/(.+?)/(?:blob|raw)/.*$`)
exp3 = regexp.MustCompile(`^(?:https?://)?github\.com/(.+?)/(.+?)/(?:info|git-).*$`)
exp4 = regexp.MustCompile(`^(?:https?://)?raw\.(?:githubusercontent|github)\.com/(.+?)/(.+?)/.+?/.+$`)
exp5 = regexp.MustCompile(`^(?:https?://)?gist\.(?:githubusercontent|github)\.com/(.+?)/.+?/.+$`)
exp6 = regexp.MustCompile(`^(?:https?://)?github\.com/(.+?)/(.+?)/tags.*$`)
exp7 = regexp.MustCompile(`^(?:https?://)?api\.github\.com/.+$`)

// Used for jsDelivr URL rewriting
expRawRewrite = regexp.MustCompile(`^((?:https?://)?raw\.(?:githubusercontent|github)\.com/.+?/.+?)/(.+?/)`)

// Used for fixing double slash issue
expSchemeSlash = regexp.MustCompile(`^https?:/+`)

// proxyClient is shared across all proxy requests for connection pooling and TLS session reuse.
proxyClient = &http.Client{
Transport: &http.Transport{
DialContext: (&net.Dialer{
Timeout:   30 * time.Second,
KeepAlive: 30 * time.Second,
}).DialContext,
MaxIdleConns:          200,
MaxIdleConnsPerHost:   20,
IdleConnTimeout:       90 * time.Second,
TLSHandshakeTimeout:   10 * time.Second,
ExpectContinueTimeout: 1 * time.Second,
ResponseHeaderTimeout: 30 * time.Second,
// DisableCompression prevents the transport from decompressing responses.
// This preserves the original Content-Encoding and Content-Length headers,
// which is essential for correct proxying of binary file downloads.
DisableCompression: true,
},
CheckRedirect: func(req *http.Request, via []*http.Request) error {
return http.ErrUseLastResponse
},
}
)

type ProxyHandler struct{}

func NewProxyHandler() *ProxyHandler {
return &ProxyHandler{}
}

func checkURL(u string) bool {
for _, exp := range []*regexp.Regexp{exp1, exp2, exp3, exp4, exp5, exp6, exp7} {
if exp.MatchString(u) {
return true
}
}
return false
}

func (h *ProxyHandler) Handle(c *gin.Context) {
rawPath := c.Param("path")
if rawPath == "" {
rawPath = c.Request.URL.Path
}

// Remove leading slash
rawPath = strings.TrimPrefix(rawPath, "/")

// Handle query parameter redirect
if q := c.Query("q"); q != "" {
c.Redirect(http.StatusMovedPermanently, "/"+q)
return
}

// Fix double slash issue
rawPath = expSchemeSlash.ReplaceAllString(rawPath, "https://")

if !checkURL(rawPath) {
c.JSON(http.StatusForbidden, gin.H{"error": "Invalid GitHub URL"})
return
}

// Validate token
token := c.GetHeader("X-XN-Token")
if token == "" {
token = c.Query("token")
}
if token == "" {
c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少访问令牌，请在请求头中添加 X-XN-Token"})
return
}

var tokenRecord models.Token
if err := database.DB.Preload("User", func(db *gorm.DB) *gorm.DB {
return db.Select("id, speed_limit")
}).Where("token = ?", token).First(&tokenRecord).Error; err != nil {
c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的访问令牌"})
return
}

if tokenRecord.IsExpired() {
c.JSON(http.StatusUnauthorized, gin.H{"error": "访问令牌已过期"})
return
}

// Handle blob -> raw conversion
if exp2.MatchString(rawPath) {
if config.C.Proxy.JsDelivr {
newURL := strings.Replace(rawPath, "/blob/", "@", 1)
newURL = strings.Replace(newURL, "github.com", "cdn.jsdelivr.net/gh", 1)
c.Redirect(http.StatusFound, newURL)
return
}
rawPath = strings.Replace(rawPath, "/blob/", "/raw/", 1)
} else if config.C.Proxy.JsDelivr && exp4.MatchString(rawPath) {
newURL := expRawRewrite.ReplaceAllString(rawPath, "$1@$2")
newURL = strings.Replace(newURL, "raw.githubusercontent.com", "cdn.jsdelivr.net/gh", 1)
newURL = strings.Replace(newURL, "raw.github.com", "cdn.jsdelivr.net/gh", 1)
c.Redirect(http.StatusFound, newURL)
return
}

// Ensure URL has scheme
if !strings.HasPrefix(rawPath, "https://") && !strings.HasPrefix(rawPath, "http://") {
rawPath = "https://" + rawPath
}

// Log download asynchronously to avoid blocking the proxy response.
userID := tokenRecord.UserID
tokenID := tokenRecord.ID
ip := c.ClientIP()
userAgent := c.Request.UserAgent()
go func() {
if err := database.DB.Create(&models.DownloadLog{
UserID:    userID,
TokenID:   tokenID,
URL:       rawPath,
IP:        ip,
UserAgent: userAgent,
}).Error; err != nil {
log.Printf("Failed to create download log: %v", err)
}
}()

// Proxy the request with user's speed limit
h.proxyRequest(c, rawPath, tokenRecord.User.SpeedLimit)
}

func (h *ProxyHandler) proxyRequest(c *gin.Context, targetURL string, speedLimit int64) {
h.doProxy(c, targetURL, 0, speedLimit)
}

func (h *ProxyHandler) doProxy(c *gin.Context, targetURL string, depth int, speedLimit int64) {
if depth > 10 {
c.String(http.StatusBadGateway, "Too many redirects")
return
}

parsedURL, err := url.Parse(targetURL)
if err != nil {
c.String(http.StatusBadRequest, "Invalid URL")
return
}

// Build query string
if c.Request.URL.RawQuery != "" && depth == 0 {
if parsedURL.RawQuery != "" {
parsedURL.RawQuery += "&" + c.Request.URL.RawQuery
} else {
parsedURL.RawQuery = c.Request.URL.RawQuery
}
// Remove our token query param
q := parsedURL.Query()
q.Del("token")
parsedURL.RawQuery = q.Encode()
}

req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, parsedURL.String(), c.Request.Body)
if err != nil {
c.String(http.StatusBadGateway, "Failed to create request")
return
}

// Copy headers
for key, values := range c.Request.Header {
lower := strings.ToLower(key)
if lower == "host" || lower == "x-xn-token" {
continue
}
for _, value := range values {
req.Header.Add(key, value)
}
}

resp, err := proxyClient.Do(req)
if err != nil {
c.String(http.StatusBadGateway, "Proxy error: "+err.Error())
return
}
defer resp.Body.Close()

// Check size limit
if resp.ContentLength > config.C.Proxy.SizeLimit {
c.Redirect(http.StatusFound, targetURL)
return
}

// Handle redirects
if location := resp.Header.Get("Location"); location != "" {
if checkURL(location) {
resp.Header.Set("Location", "/"+location)
} else {
h.doProxy(c, location, depth+1, speedLimit)
return
}
}

// Set response headers. Assign the value slice directly to preserve
// multi-value headers (e.g. Set-Cookie, Vary) correctly.
respHdr := c.Writer.Header()
for key, values := range resp.Header {
lower := strings.ToLower(key)
if lower == "content-security-policy" ||
lower == "content-security-policy-report-only" ||
lower == "clear-site-data" {
continue
}
respHdr[key] = values
}

respHdr.Set("Access-Control-Allow-Origin", "*")
respHdr.Set("Access-Control-Expose-Headers", "*")

c.Status(resp.StatusCode)

// Choose reader and buffer based on whether rate limiting is needed
var reader io.Reader = resp.Body
var buf []byte

var userLimiter *tokenBucket
if speedLimit > 0 {
userLimiter = newTokenBucket(speedLimit)
}

if userLimiter != nil || getGlobalLimiter() != nil {
reader = &throttledReader{
r:    resp.Body,
user: userLimiter,
ctx:  c.Request.Context(),
}
buf = make([]byte, proxyBufSize)
} else {
buf = make([]byte, proxyCopyBuf)
}

if _, err := io.CopyBuffer(c.Writer, reader, buf); err != nil {
// Client probably disconnected, just log
log.Printf("Proxy stream error: %v", err)
}
}
