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
)

func main() {
	// 主流程：加载配置、连接数据库、启动 HTTP 服务并等待退出信号
	logger := log.New(os.Stdout, "api ", log.LstdFlags|log.LUTC)
	cfg := loadConfig()

	db, err := openDB(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Fatalf("db connect failed: %v", err)
	}
	defer db.Close()

	app := newApp(cfg, db, logger)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      app.routes(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		// 启动 HTTP 服务，非正常关闭才记录错误
		logger.Printf("listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server error: %v", err)
		}
	}()

	// 监听系统信号，触发优雅退出
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Println("shutting down")
	// 给予超时时间完成正在处理的请求
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("shutdown error: %v", err)
	}
}
