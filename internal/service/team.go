package service

import (
	"context"
	"errors"

	"github.com/fatykhovar/task-manager/internal/model"
	"github.com/fatykhovar/task-manager/internal/repository"
)

var (
	ErrNotTeamMember     = errors.New("not a team member")
	ErrInsufficientRole  = errors.New("insufficient permissions")
	ErrTeamNotFound      = errors.New("team not found")
)

type TeamService struct {
	teams *repository.TeamRepository
	users *repository.UserRepository
}

func NewTeamService(teams *repository.TeamRepository, users *repository.UserRepository) *TeamService {
	return &TeamService{teams: teams, users: users}
}

func (s *TeamService) CreateTeam(ctx context.Context, creatorID int64, name, description string) (*model.Team, error) {
	t := &model.Team{
		Name:        name,
		Description: description,
		CreatedBy:   creatorID,
	}
	if err := s.teams.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TeamService) ListTeams(ctx context.Context, userID int64) ([]*model.Team, error) {
	return s.teams.ListByUserID(ctx, userID)
}

func (s *TeamService) InviteUser(ctx context.Context, inviterID, teamID int64, inviteeEmail string, role model.TeamRole) error {
	// Check inviter permissions
	member, err := s.teams.GetMember(ctx, teamID, inviterID)
	if err != nil {
		return err
	}
	if member == nil {
		return ErrNotTeamMember
	}
	if member.Role != model.RoleOwner && member.Role != model.RoleAdmin {
		return ErrInsufficientRole
	}

	// Find invitee
	invitee, err := s.users.FindByEmail(ctx, inviteeEmail)
	if err != nil {
		return err
	}
	if invitee == nil {
		return ErrUserNotFound
	}

	return s.teams.AddMember(ctx, &model.TeamMember{
		UserID: invitee.ID,
		TeamID: teamID,
		Role:   role,
	})
}

func (s *TeamService) GetMember(ctx context.Context, teamID, userID int64) (*model.TeamMember, error) {
	return s.teams.GetMember(ctx, teamID, userID)
}
