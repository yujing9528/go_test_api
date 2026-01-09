package database

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Open(dsn string, logger *log.Logger) (*sql.DB, error) {
	// 初始化数据库连接池并做连通性检查
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	// 连接池参数按学习场景做一个合理默认
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// 在启动阶段快速失败，避免运行时才暴露问题
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	logger.Println("database connected")
	return db, nil
}
