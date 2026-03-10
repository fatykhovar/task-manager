package model

import "time"

// User represents an application user
type User struct {
	ID           int64     `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// Team represents a team
type Team struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedBy   int64     `json:"created_by" db:"created_by"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// TeamRole is the role of a user within a team
type TeamRole string

const (
	RoleOwner  TeamRole = "owner"
	RoleAdmin  TeamRole = "admin"
	RoleMember TeamRole = "member"
)

// TeamMember represents the many-to-many between users and teams
type TeamMember struct {
	UserID   int64     `json:"user_id" db:"user_id"`
	TeamID   int64     `json:"team_id" db:"team_id"`
	Role     TeamRole  `json:"role" db:"role"`
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

// TaskPriority represents task priority
type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

// Task represents a task
type Task struct {
	ID          int64        `json:"id" db:"id"`
	Title       string       `json:"title" db:"title"`
	Description string       `json:"description" db:"description"`
	Status      TaskStatus   `json:"status" db:"status"`
	Priority    TaskPriority `json:"priority" db:"priority"`
	AssigneeID  *int64       `json:"assignee_id,omitempty" db:"assignee_id"`
	TeamID      int64        `json:"team_id" db:"team_id"`
	CreatedBy   int64        `json:"created_by" db:"created_by"`
	DueDate     *time.Time   `json:"due_date,omitempty" db:"due_date"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
}

// TaskHistory represents an audit log entry for a task
type TaskHistory struct {
	ID        int64     `json:"id" db:"id"`
	TaskID    int64     `json:"task_id" db:"task_id"`
	ChangedBy int64     `json:"changed_by" db:"changed_by"`
	FieldName string    `json:"field_name" db:"field_name"`
	OldValue  string    `json:"old_value" db:"old_value"`
	NewValue  string    `json:"new_value" db:"new_value"`
	ChangedAt time.Time `json:"changed_at" db:"changed_at"`
}

// TaskComment represents a comment on a task
type TaskComment struct {
	ID        int64     `json:"id" db:"id"`
	TaskID    int64     `json:"task_id" db:"task_id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Content   string    `json:"content" db:"content"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TaskFilter for querying tasks
type TaskFilter struct {
	TeamID     *int64
	Status     *TaskStatus
	AssigneeID *int64
	Page       int
	PageSize   int
}

// TeamStats for analytics
type TeamStats struct {
	TeamID       int64  `json:"team_id" db:"team_id"`
	TeamName     string `json:"team_name" db:"team_name"`
	MemberCount  int    `json:"member_count" db:"member_count"`
	DoneLastWeek int    `json:"done_last_week" db:"done_last_week"`
}

// TopUser for analytics
type TopUser struct {
	UserID    int64  `json:"user_id" db:"user_id"`
	Username  string `json:"username" db:"username"`
	TeamID    int64  `json:"team_id" db:"team_id"`
	TeamName  string `json:"team_name" db:"team_name"`
	TaskCount int    `json:"task_count" db:"task_count"`
	Rank      int    `json:"rank" db:"rank"`
}

// Pagination response
type PaginatedResponse struct {
	Data     interface{} `json:"data"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}
