# Go + PostgreSQL 微服务学习项目

这是一个按功能模块拆分的 Go + PostgreSQL REST API 示例，用于练习路由、JSON 处理、CRUD 以及优雅退出。

当前包含两个服务：
- `todo-api`：Todo 的增删改查
- `stats-api`：Todo 的统计汇总

## 目录结构

- `cmd/todo-api`：Todo 服务入口
- `cmd/stats-api`：统计服务入口
- `internal/config`：配置读取
- `internal/database`：数据库连接
- `internal/todo`：Todo 领域逻辑
- `internal/stats`：统计领域逻辑
- `migrations`：SQL 初始化脚本
- `docker-compose.yml`：本地 PostgreSQL
- `.env.example`：环境变量示例

## 快速开始

1) 启动 PostgreSQL

```bash
docker compose up -d
```

2) 初始化表结构

```bash
psql "postgres://postgres:123456@localhost:5432/postgres?sslmode=disable" -c "CREATE DATABASE go_api;"
psql "postgres://postgres:123456@localhost:5432/go_api?sslmode=disable" -f migrations/001_init.sql
```

如果你的数据库密码不是 `123456`，请替换连接串中的密码。

3) 启动服务

Todo 服务（默认端口 8081）：

```bash
go run ./cmd/todo-api
```

统计服务（默认端口 8082）：

```bash
go run ./cmd/stats-api
```

## 环境变量

复制 `.env.example` 并按需修改：

- `ADDR`：服务监听地址（每个服务都可以单独设置）
- `DATABASE_URL`：PostgreSQL 连接串
- `READ_TIMEOUT`/`WRITE_TIMEOUT`/`IDLE_TIMEOUT`：可选，格式如 `5s` 或 `1m`

例如分别指定端口：

```bash
ADDR=:8081 go run ./cmd/todo-api
ADDR=:8082 go run ./cmd/stats-api
```

## API 示例

Todo 服务：

```bash
curl http://localhost:8081/health
```

```bash
curl -X POST http://localhost:8081/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"learn sql","done":false}'
```

```bash
curl http://localhost:8081/todos
```

统计服务：

```bash
curl http://localhost:8082/health
```

```bash
curl http://localhost:8082/stats
```
