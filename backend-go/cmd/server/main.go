package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/config"
	"pennypickbackend/internal/database"
	"pennypickbackend/internal/handler"
	"pennypickbackend/internal/middleware"
)

func main() {
	cfg := config.Load()

	h, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() {
		if err := h.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	db := h.DB()
	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	database.InitAdmin(db, cfg)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(middleware.CORS())

	auth := middleware.NewAuth(cfg, db)
	hh := handler.New(db, cfg, auth)
	hh.RegisterRoutes(r)

	// 加密模式下每 5 分钟回写一次 .enc，缩小崩溃丢失窗口
	stopFlush := make(chan struct{})
	defer close(stopFlush)
	go h.AutoFlush(5*time.Minute, stopFlush)

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}
	go func() {
		log.Printf("%s 已启动，监听 %s", cfg.ProjectName, addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("run server: %v", err)
		}
	}()

	// 等待退出信号，优雅关闭并回写加密数据库
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("正在停止服务并回写加密数据库...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
