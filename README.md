# Go + PostgreSQL Web API 学习项目

这是一个小而完整的 Go + PostgreSQL REST API 示例，用于练习路由、JSON 处理、CRUD 以及优雅退出。

## 目录结构

- `cmd/api`：应用入口与 HTTP 处理
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
psql "postgres://postgres:postgres@localhost:5432/go_api?sslmode=disable" -f migrations/001_init.sql
```

3) 运行服务

```bash
go run ./cmd/api
```

默认地址：`http://localhost:8080`

## 环境变量

复制 `.env.example` 并按需修改：

- `ADDR`：服务监听地址，默认 `:8080`
- `DATABASE_URL`：PostgreSQL 连接串
- `READ_TIMEOUT`/`WRITE_TIMEOUT`/`IDLE_TIMEOUT`：可选，格式如 `5s` 或 `1m`

## API 示例

健康检查：

```bash
curl http://localhost:8080/health
```

创建：

```bash
curl -X POST http://localhost:8080/todos \
  -H "Content-Type: application/json" \
  -d '{"title":"learn sql","done":false}'
```

列表：

```bash
curl http://localhost:8080/todos
```

查询：

```bash
curl http://localhost:8080/todos/1
```

更新：

```bash
curl -X PUT http://localhost:8080/todos/1 \
  -H "Content-Type: application/json" \
  -d '{"done":true}'
```

删除：

```bash
curl -X DELETE http://localhost:8080/todos/1
```
