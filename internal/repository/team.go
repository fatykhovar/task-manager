package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fatykhovar/task-manager/internal/model"
)

type TeamRepository struct {
	db *sql.DB
}

func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

func (r *TeamRepository) Create(ctx context.Context, t *model.Team) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const q = `INSERT INTO teams (name, description, created_by) VALUES ($1, $2, $3) RETURNING id`
	if err := tx.QueryRowContext(ctx, q, t.Name, t.Description, t.CreatedBy).Scan(&t.ID); err != nil {
		return fmt.Errorf("create team: %w", err)
	}

	// Creator becomes owner
	const mq = `INSERT INTO team_members (user_id, team_id, role) VALUES ($1, $2, 'owner')`
	if _, err := tx.ExecContext(ctx, mq, t.CreatedBy, t.ID); err != nil {
		return fmt.Errorf("add owner to team: %w", err)
	}

	return tx.Commit()
}

func (r *TeamRepository) ListByUserID(ctx context.Context, userID int64) ([]*model.Team, error) {
	const q = `
		SELECT t.id, t.name, t.description, t.created_by, t.created_at
		FROM teams t
		INNER JOIN team_members tm ON tm.team_id = t.id
		WHERE tm.user_id = $1
		ORDER BY t.created_at DESC`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	defer rows.Close()

	var teams []*model.Team
	for rows.Next() {
		t := &model.Team{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (r *TeamRepository) GetMember(ctx context.Context, teamID, userID int64) (*model.TeamMember, error) {
	const q = `SELECT user_id, team_id, role, joined_at FROM team_members WHERE team_id = $1 AND user_id = $2`
	m := &model.TeamMember{}
	err := r.db.QueryRowContext(ctx, q, teamID, userID).Scan(&m.UserID, &m.TeamID, &m.Role, &m.JoinedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get member: %w", err)
	}
	return m, nil
}

func (r *TeamRepository) AddMember(ctx context.Context, m *model.TeamMember) error {
	const q = `
		INSERT INTO team_members (user_id, team_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, team_id) DO UPDATE SET role = EXCLUDED.role`
	_, err := r.db.ExecContext(ctx, q, m.UserID, m.TeamID, m.Role)
	return err
}

func (r *TeamRepository) GetByID(ctx context.Context, id int64) (*model.Team, error) {
	const q = `SELECT id, name, description, created_by, created_at FROM teams WHERE id = $1`
	t := &model.Team{}
	err := r.db.QueryRowContext(ctx, q, id).Scan(&t.ID, &t.Name, &t.Description, &t.CreatedBy, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get team by id: %w", err)
	}
	return t, nil
}

// TeamStats: JOIN 3+ tables + aggregation
func (r *TeamRepository) GetTeamStats(ctx context.Context) ([]*model.TeamStats, error) {
	const q = `
		SELECT
			t.id AS team_id,
			t.name AS team_name,
			COUNT(DISTINCT tm.user_id) AS member_count,
			COUNT(DISTINCT CASE WHEN tk.status = 'done' AND tk.updated_at >= NOW() - INTERVAL '7 days' THEN tk.id END) AS done_last_week
		FROM teams t
		LEFT JOIN team_members tm ON tm.team_id = t.id
		LEFT JOIN tasks tk ON tk.team_id = t.id
		GROUP BY t.id, t.name
		ORDER BY t.id`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get team stats: %w", err)
	}
	defer rows.Close()

	var stats []*model.TeamStats
	for rows.Next() {
		s := &model.TeamStats{}
		if err := rows.Scan(&s.TeamID, &s.TeamName, &s.MemberCount, &s.DoneLastWeek); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// TopUsersByTeam: window function ROW_NUMBER() — top 3 by tasks created per team this month
func (r *TeamRepository) GetTopUsersByTeam(ctx context.Context) ([]*model.TopUser, error) {
	const q = `
		WITH ranked AS (
			SELECT
				t.id AS team_id,
				t.name AS team_name,
				u.id AS user_id,
				u.username,
				COUNT(tk.id) AS task_count,
				ROW_NUMBER() OVER (PARTITION BY t.id ORDER BY COUNT(tk.id) DESC) AS rn
			FROM teams t
			JOIN team_members tm ON tm.team_id = t.id
			JOIN users u ON u.id = tm.user_id
			LEFT JOIN tasks tk ON tk.created_by = u.id
				AND tk.team_id = t.id
				AND tk.created_at >= DATE_TRUNC('month', NOW())
			GROUP BY t.id, t.name, u.id, u.username
		)
		SELECT user_id, username, team_id, team_name, task_count, rn AS rank
		FROM ranked
		WHERE rn <= 3
		ORDER BY team_id, rn`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get top users: %w", err)
	}
	defer rows.Close()

	var users []*model.TopUser
	for rows.Next() {
		u := &model.TopUser{}
		if err := rows.Scan(&u.UserID, &u.Username, &u.TeamID, &u.TeamName, &u.TaskCount, &u.Rank); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// IntegrityCheck: tasks where assignee is not a member of the task's team
func (r *TeamRepository) GetIntegrityViolations(ctx context.Context) ([]*model.Task, error) {
	const q = `
		SELECT tk.id, tk.title, tk.status, tk.priority, tk.assignee_id, tk.team_id, tk.created_by, tk.created_at, tk.updated_at
		FROM tasks tk
		WHERE tk.assignee_id IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1 FROM team_members tm
			WHERE tm.user_id = tk.assignee_id
			  AND tm.team_id = tk.team_id
		  )`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("get integrity violations: %w", err)
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		t := &model.Task{}
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.Priority, &t.AssigneeID, &t.TeamID, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
