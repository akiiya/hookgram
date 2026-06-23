package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hookgram/internal/config"
	"hookgram/internal/database"
	"hookgram/internal/httpapi"
	"hookgram/internal/repository"
	"hookgram/internal/service"
	"hookgram/internal/telegram"
)

func main() {
	cfgManager, err := config.LoadOrCreate(config.PathFromEnv())
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	db, err := database.Open(cfgManager.Current())
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	repo := repository.New(db)
	telegramClient := telegram.NewClient(cfgManager)
	services := service.NewContainer(cfgManager, repo, telegramClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	services.Bot.Start(ctx)

	router := httpapi.NewRouter(cfgManager, services)
	cfg := cfgManager.Current()
	addr := cfg.App.ListenAddr()
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Hookgram 已启动: http://%s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务异常退出: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP 服务关闭失败: %v", err)
	}
}
