package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/fatykhovar/task-manager/internal/cache"
	"github.com/fatykhovar/task-manager/internal/model"
	"github.com/fatykhovar/task-manager/internal/repository"
	"go.uber.org/zap"
)

var (
	ErrTaskNotFound      = errors.New("task not found")
	ErrUnauthorized      = errors.New("unauthorized")
)

type TaskService struct {
	tasks     *repository.TaskRepository
	teams     *repository.TeamRepository
	taskCache *cache.TaskCache
	logger    *zap.Logger
}

func NewTaskService(tasks *repository.TaskRepository, teams *repository.TeamRepository, taskCache *cache.TaskCache, logger *zap.Logger) *TaskService {
	return &TaskService{tasks: tasks, teams: teams, taskCache: taskCache, logger: logger}
}

func (s *TaskService) CreateTask(ctx context.Context, creatorID int64, t *model.Task) (*model.Task, error) {
	// Must be a team member
	member, err := s.teams.GetMember(ctx, t.TeamID, creatorID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrNotTeamMember
	}

	t.CreatedBy = creatorID
	t.Status = model.StatusTodo

	if err := s.tasks.Create(ctx, t); err != nil {
		return nil, err
	}

	// Invalidate cache for the team
	s.taskCache.InvalidateTeam(ctx, t.TeamID)

	return t, nil
}

func (s *TaskService) ListTasks(ctx context.Context, f model.TaskFilter) ([]*model.Task, int, error) {
	// Try cache when filtering by team
	if f.TeamID != nil && f.Status != nil {
		statusStr := string(*f.Status)
		if cached, ok := s.taskCache.GetTeamTasks(ctx, *f.TeamID, statusStr, f.Page); ok {
			s.logger.Debug("tasks cache hit", zap.Int64("team_id", *f.TeamID))
			return cached, len(cached), nil
		}
	}

	tasks, total, err := s.tasks.List(ctx, f)
	if err != nil {
		return nil, 0, err
	}

	// Store in cache
	if f.TeamID != nil && f.Status != nil {
		statusStr := string(*f.Status)
		s.taskCache.SetTeamTasks(ctx, *f.TeamID, statusStr, f.Page, tasks)
	}

	return tasks, total, nil
}

func (s *TaskService) UpdateTask(ctx context.Context, userID, taskID int64, updates *model.Task) (*model.Task, error) {
	existing, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrTaskNotFound
	}

	// Check user is a team member
	member, err := s.teams.GetMember(ctx, existing.TeamID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrUnauthorized
	}

	// Build history
	changes := buildChanges(taskID, userID, existing, updates)

	// Apply updates
	existing.Title = updates.Title
	existing.Description = updates.Description
	existing.Status = updates.Status
	existing.Priority = updates.Priority
	existing.AssigneeID = updates.AssigneeID
	existing.DueDate = updates.DueDate

	if err := s.tasks.Update(ctx, existing, changes); err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}

	s.taskCache.InvalidateTeam(ctx, existing.TeamID)

	return existing, nil
}

func (s *TaskService) GetHistory(ctx context.Context, userID, taskID int64) ([]*model.TaskHistory, error) {
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrTaskNotFound
	}

	member, err := s.teams.GetMember(ctx, task.TeamID, userID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, ErrUnauthorized
	}

	return s.tasks.GetHistory(ctx, taskID)
}

func buildChanges(taskID, changedBy int64, old, new *model.Task) []model.TaskHistory {
	var changes []model.TaskHistory
	add := func(field, oldVal, newVal string) {
		if oldVal != newVal {
			changes = append(changes, model.TaskHistory{
				TaskID:    taskID,
				ChangedBy: changedBy,
				FieldName: field,
				OldValue:  oldVal,
				NewValue:  newVal,
			})
		}
	}

	add("title", old.Title, new.Title)
	add("description", old.Description, new.Description)
	add("status", string(old.Status), string(new.Status))
	add("priority", string(old.Priority), string(new.Priority))

	oldAssignee := ""
	newAssignee := ""
	if old.AssigneeID != nil {
		oldAssignee = fmt.Sprintf("%d", *old.AssigneeID)
	}
	if new.AssigneeID != nil {
		newAssignee = fmt.Sprintf("%d", *new.AssigneeID)
	}
	add("assignee_id", oldAssignee, newAssignee)

	return changes
}
