package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/fatykhovar/task-manager/internal/model"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, t *model.Task) error {
	const q = `
		INSERT INTO tasks (title, description, status, priority, assignee_id, team_id, created_by, due_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRowContext(ctx, q, t.Title, t.Description, t.Status, t.Priority,
		t.AssigneeID, t.TeamID, t.CreatedBy, t.DueDate).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *TaskRepository) GetByID(ctx context.Context, id int64) (*model.Task, error) {
	const q = `
		SELECT id, title, description, status, priority, assignee_id, team_id, created_by, due_date, created_at, updated_at
		FROM tasks WHERE id = $1`
	t := &model.Task{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.AssigneeID, &t.TeamID, &t.CreatedBy, &t.DueDate, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

func (r *TaskRepository) List(ctx context.Context, f model.TaskFilter) ([]*model.Task, int, error) {
	args := []interface{}{}
	conditions := []string{}
	argN := 1

	if f.TeamID != nil {
		conditions = append(conditions, fmt.Sprintf("team_id = $%d", argN))
		args = append(args, *f.TeamID)
		argN++
	}
	if f.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argN))
		args = append(args, *f.Status)
		argN++
	}
	if f.AssigneeID != nil {
		conditions = append(conditions, fmt.Sprintf("assignee_id = $%d", argN))
		args = append(args, *f.AssigneeID)
		argN++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM tasks %s", where)
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}

	if f.PageSize <= 0 {
		f.PageSize = 20
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.PageSize

	q := fmt.Sprintf(`
		SELECT id, title, description, status, priority, assignee_id, team_id, created_by, due_date, created_at, updated_at
		FROM tasks %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argN, argN+1)

	args = append(args, f.PageSize, offset)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		t := &model.Task{}
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
			&t.AssigneeID, &t.TeamID, &t.CreatedBy, &t.DueDate, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, t)
	}
	return tasks, total, rows.Err()
}

func (r *TaskRepository) Update(ctx context.Context, t *model.Task, changes []model.TaskHistory) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const q = `
		UPDATE tasks SET title=$1, description=$2, status=$3, priority=$4, assignee_id=$5, due_date=$6, updated_at=NOW()
		WHERE id = $7`
	if _, err := tx.ExecContext(ctx, q, t.Title, t.Description, t.Status, t.Priority,
		t.AssigneeID, t.DueDate, t.ID); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	const hq = `INSERT INTO task_history (task_id, changed_by, field_name, old_value, new_value) VALUES ($1, $2, $3, $4, $5)`
	for _, ch := range changes {
		if _, err := tx.ExecContext(ctx, hq, ch.TaskID, ch.ChangedBy, ch.FieldName, ch.OldValue, ch.NewValue); err != nil {
			return fmt.Errorf("insert history: %w", err)
		}
	}

	return tx.Commit()
}

func (r *TaskRepository) GetHistory(ctx context.Context, taskID int64) ([]*model.TaskHistory, error) {
	const q = `
		SELECT id, task_id, changed_by, field_name, old_value, new_value, changed_at
		FROM task_history
		WHERE task_id = $1
		ORDER BY changed_at DESC`

	rows, err := r.db.QueryContext(ctx, q, taskID)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	defer rows.Close()

	var history []*model.TaskHistory
	for rows.Next() {
		h := &model.TaskHistory{}
		if err := rows.Scan(&h.ID, &h.TaskID, &h.ChangedBy, &h.FieldName, &h.OldValue, &h.NewValue, &h.ChangedAt); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, rows.Err()
}
