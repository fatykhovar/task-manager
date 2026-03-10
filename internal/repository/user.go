package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fatykhovar/task-manager/internal/model"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	const q = `INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id, created_at`
	return r.db.QueryRowContext(ctx, q, u.Username, u.Email, u.PasswordHash).Scan(&u.ID, &u.CreatedAt)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	const q = `SELECT id, username, email, password_hash, created_at FROM users WHERE email = $1`
	u := &model.User{}
	err := r.db.QueryRowContext(ctx, q, email).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	const q = `SELECT id, username, email, password_hash, created_at FROM users WHERE id = $1`
	u := &model.User{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	const q = `SELECT id, username, email, created_at FROM users WHERE username = $1`
	u := &model.User{}
	err := r.db.QueryRowContext(ctx, q, username).Scan(
		&u.ID, &u.Username, &u.Email, &u.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user by username: %w", err)
	}
	return u, nil
}
