package models

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           string
	Email        string
	Name         string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func CreateUser(ctx context.Context, pool *pgxpool.Pool, email, name, passwordHash string) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, password_hash, created_at, updated_at
	`, email, name, passwordHash).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func FindUserByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, created_at, updated_at
		FROM users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func FindUserByID(ctx context.Context, pool *pgxpool.Pool, id string) (*User, error) {
	u := &User{}
	err := pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, created_at, updated_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func CreateSession(ctx context.Context, pool *pgxpool.Pool, id, userID string, expiresAt time.Time) (*Session, error) {
	s := &Session{}
	err := pool.QueryRow(ctx, `
		INSERT INTO sessions (id, user_id, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, expires_at, created_at
	`, id, userID, expiresAt).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func FindSession(ctx context.Context, pool *pgxpool.Pool, id string) (*Session, error) {
	s := &Session{}
	err := pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, created_at
		FROM sessions WHERE id = $1
	`, id).Scan(&s.ID, &s.UserID, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

func DeleteSession(ctx context.Context, pool *pgxpool.Pool, id string) error {
	_, err := pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}
