package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fatykhovar/task-manager/internal/model"
)

type CommentRepository struct {
	db *sql.DB
}

func NewCommentRepository(db *sql.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) Create(ctx context.Context, c *model.TaskComment) error {
	const q = `INSERT INTO task_comments (task_id, user_id, content) VALUES ($1, $2, $3) RETURNING id, created_at`
	return r.db.QueryRowContext(ctx, q, c.TaskID, c.UserID, c.Content).Scan(&c.ID, &c.CreatedAt)
}

func (r *CommentRepository) ListByTask(ctx context.Context, taskID int64) ([]*model.TaskComment, error) {
	const q = `
		SELECT id, task_id, user_id, content, created_at
		FROM task_comments
		WHERE task_id = $1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, q, taskID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()

	var comments []*model.TaskComment
	for rows.Next() {
		c := &model.TaskComment{}
		if err := rows.Scan(&c.ID, &c.TaskID, &c.UserID, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (r *CommentRepository) TaskExists(ctx context.Context, taskID int64) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE id = $1", taskID).Scan(&count)
	return count > 0, err
}

// Ensure user is member of task's team before commenting
func (r *CommentRepository) CanComment(ctx context.Context, userID, taskID int64) (bool, error) {
	const q = `
		SELECT COUNT(*)
		FROM tasks tk
		JOIN team_members tm ON tm.team_id = tk.team_id AND tm.user_id = $1
		WHERE tk.id = $2`
	var count int
	err := r.db.QueryRowContext(ctx, q, userID, taskID).Scan(&count)
	return count > 0, err
}
