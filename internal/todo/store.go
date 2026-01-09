package main

import (
	"context"
	"database/sql"
)

type todoStore struct {
	db *sql.DB
}

func newTodoStore(db *sql.DB) *todoStore {
	// 数据访问层封装
	return &todoStore{db: db}
}

func (s *todoStore) List(ctx context.Context) ([]Todo, error) {
	// 查询全部 todo
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, done, created_at, updated_at
		FROM todos
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var todo Todo
		if err := rows.Scan(&todo.ID, &todo.Title, &todo.Done, &todo.CreatedAt, &todo.UpdatedAt); err != nil {
			return nil, err
		}
		todos = append(todos, todo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return todos, nil
}

func (s *todoStore) Get(ctx context.Context, id int64) (Todo, error) {
	// 按 ID 查询
	var todo Todo
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, done, created_at, updated_at
		FROM todos
		WHERE id = $1
	`, id)
	if err := row.Scan(&todo.ID, &todo.Title, &todo.Done, &todo.CreatedAt, &todo.UpdatedAt); err != nil {
		return Todo{}, err
	}
	return todo, nil
}

func (s *todoStore) Create(ctx context.Context, input createTodoRequest) (Todo, error) {
	// 新建 todo
	var todo Todo
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO todos (title, done)
		VALUES ($1, $2)
		RETURNING id, title, done, created_at, updated_at
	`, input.Title, input.Done)
	if err := row.Scan(&todo.ID, &todo.Title, &todo.Done, &todo.CreatedAt, &todo.UpdatedAt); err != nil {
		return Todo{}, err
	}
	return todo, nil
}

func (s *todoStore) Update(ctx context.Context, id int64, input updateTodoRequest) (Todo, error) {
	// 更新 todo（部分字段）
	var todo Todo
	row := s.db.QueryRowContext(ctx, `
		UPDATE todos
		SET title = COALESCE($1, title),
			done = COALESCE($2, done),
			updated_at = NOW()
		WHERE id = $3
		RETURNING id, title, done, created_at, updated_at
	`, nullableString(input.Title), nullableBool(input.Done), id)
	if err := row.Scan(&todo.ID, &todo.Title, &todo.Done, &todo.CreatedAt, &todo.UpdatedAt); err != nil {
		return Todo{}, err
	}
	return todo, nil
}

func (s *todoStore) Delete(ctx context.Context, id int64) (bool, error) {
	// 删除 todo
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM todos
		WHERE id = $1
	`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
