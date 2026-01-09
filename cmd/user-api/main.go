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

	"go_test/internal/config"
	"go_test/internal/database"
	"go_test/internal/user"
)

func main() {
	logger := log.New(os.Stdout, "user-api ", log.LstdFlags|log.LUTC)
	cfg := config.Load(":8083")

	db, err := database.Open(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Fatalf("db connect failed: %v", err)
	}
	defer db.Close()

	handler := user.NewHandler(user.NewStore(db), logger)

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler.Routes(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		logger.Printf("listening on %s", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("shutdown error: %v", err)
	}
}
