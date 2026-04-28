package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/uuuuupdate/gh-proxy-auth/internal/config"
	"github.com/uuuuupdate/gh-proxy-auth/internal/database"
	"github.com/uuuuupdate/gh-proxy-auth/internal/frontend"
	"github.com/uuuuupdate/gh-proxy-auth/internal/handlers"
	"github.com/uuuuupdate/gh-proxy-auth/internal/router"
	"github.com/uuuuupdate/gh-proxy-auth/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// Register version function for the update handler.
	handlers.SetVersionFunc(func() string { return Version })

	// Load config
	if err := config.Load(*configPath); err != nil {
		log.Fatalf("加载配置文件失败: %v", err)
	}

	// Initialize database
	if err := database.Init(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	// Initialize global bandwidth limiter from settings.
	handlers.InitGlobalLimiter()

	// Initialize WebAuthn
	if err := service.InitWebAuthn(); err != nil {
		log.Fatalf("初始化 WebAuthn 失败: %v", err)
	}

	// Initialize embedded frontend
	if err := frontend.Init(); err != nil {
		log.Printf("警告: 加载前端资源失败: %v", err)
	}

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()

	// Setup routes
	router.Setup(engine)

	addr := fmt.Sprintf("%s:%d", config.C.Server.Host, config.C.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	// Start server in background.
	go func() {
		log.Printf("服务启动在 %s (版本: %s)", addr, Version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("启动服务器失败: %v", err)
		}
	}()

	// Wait for OS signal or restart request.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	var doRestart bool
	select {
	case <-quit:
		log.Println("收到关闭信号，正在优雅退出...")
	case <-handlers.RestartChan:
		doRestart = true
		log.Println("收到重启信号，正在优雅重启...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("服务器关闭超时: %v", err)
	}

	if doRestart {
		log.Println("正在启动新版本...")
		if err := handlers.DoRestart(); err != nil {
			// syscall.Exec failed – fall back to plain exit so the process
			// manager (systemd, etc.) can restart us.
			log.Printf("exec 重启失败: %v，将直接退出由进程管理器重启", err)
			os.Exit(0)
		}
	}
}
