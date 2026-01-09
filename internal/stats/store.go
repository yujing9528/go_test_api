package stats

import (
	"context"
	"database/sql"
)

type Summary struct {
	Total int64 `json:"total"`
	Done  int64 `json:"done"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	// 数据访问层封装
	return &Store{db: db}
}

func (s *Store) Summary(ctx context.Context) (Summary, error) {
	// 汇总 todo 统计信息
	var summary Summary
	row := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN done THEN 1 ELSE 0 END), 0) AS done
		FROM todos
	`)
	if err := row.Scan(&summary.Total, &summary.Done); err != nil {
		return Summary{}, err
	}
	return summary, nil
}
