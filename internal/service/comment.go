package service

import (
	"context"
	"errors"

	"github.com/fatykhovar/task-manager/internal/model"
	"github.com/fatykhovar/task-manager/internal/repository"
)

var ErrCannotComment = errors.New("user cannot comment on this task")

type CommentService struct {
	comments *repository.CommentRepository
	tasks    *repository.TaskRepository
}

func NewCommentService(comments *repository.CommentRepository, tasks *repository.TaskRepository) *CommentService {
	return &CommentService{comments: comments, tasks: tasks}
}

func (s *CommentService) AddComment(ctx context.Context, userID, taskID int64, content string) (*model.TaskComment, error) {
	canComment, err := s.comments.CanComment(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}
	if !canComment {
		return nil, ErrCannotComment
	}

	c := &model.TaskComment{
		TaskID:  taskID,
		UserID:  userID,
		Content: content,
	}
	if err := s.comments.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}
