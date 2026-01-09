package user

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

var ErrEmailExists = errors.New("email already exists")

const uniqueViolationCode = "23505"

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateUser(ctx context.Context, email, name, passwordHash string) (User, error) {
	var user User
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, password_hash, created_at, updated_at
	`, email, name, passwordHash)
	if err := row.Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailExists
		}
		return User{}, err
	}
	return user, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email)
	if err := row.Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (User, error) {
	var user User
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id)
	if err := row.Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) UpdateUser(ctx context.Context, id int64, input UpdateProfileRequest) (User, error) {
	var user User
	row := s.db.QueryRowContext(ctx, `
		UPDATE users
		SET email = COALESCE($1, email),
			name = COALESCE($2, name),
			updated_at = NOW()
		WHERE id = $3
		RETURNING id, email, name, password_hash, created_at, updated_at
	`, nullableString(input.Email), nullableString(input.Name), id)
	if err := row.Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrEmailExists
		}
		return User{}, err
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64, token string, expiresAt time.Time) (Session, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO user_sessions (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING token, expires_at
	`, userID, token, expiresAt)

	var session Session
	if err := row.Scan(&session.Token, &session.ExpiresAt); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Store) GetUserBySessionToken(ctx context.Context, token string) (User, error) {
	var user User
	row := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, u.name, u.password_hash, u.created_at, u.updated_at
		FROM user_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.expires_at > NOW()
	`, token)
	if err := row.Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) CreatePasswordReset(ctx context.Context, userID int64, token string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO password_resets (user_id, token, expires_at)
		VALUES ($1, $2, $3)
	`, userID, token, expiresAt)
	return err
}

func (s *Store) ConsumePasswordReset(ctx context.Context, token, newHash string) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	var userID int64
	row := tx.QueryRowContext(ctx, `
		SELECT user_id
		FROM password_resets
		WHERE token = $1 AND expires_at > NOW()
		FOR UPDATE
	`, token)
	if err := row.Scan(&userID); err != nil {
		return User{}, err
	}

	var user User
	row = tx.QueryRowContext(ctx, `
		UPDATE users
		SET password_hash = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, email, name, password_hash, created_at, updated_at
	`, newHash, userID)
	if err := row.Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return User{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM password_resets
		WHERE token = $1
	`, token); err != nil {
		return User{}, err
	}

	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	return user, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == uniqueViolationCode
	}
	return false
}
