package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/fatykhovar/task-manager/internal/config"
	"github.com/fatykhovar/task-manager/internal/model"
	"github.com/fatykhovar/task-manager/internal/service"
)

type MockUserRepository struct {
	users  map[string]*model.User
	byID   map[int64]*model.User
	nextID int64
}

func NewMockUserRepo() *MockUserRepository {
	return &MockUserRepository{
		users:  make(map[string]*model.User),
		byID:   make(map[int64]*model.User),
		nextID: 1,
	}
}

func (m *MockUserRepository) Create(ctx context.Context, u *model.User) error {
	m.nextID++
	u.ID = m.nextID
	m.users[u.Email] = u
	m.byID[u.ID] = u
	return nil
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *MockUserRepository) FindByID(ctx context.Context, id int64) (*model.User, error) {
	u, ok := m.byID[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *MockUserRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, nil
}

func newTestAuthService(repo *MockUserRepository) *service.AuthService {
	return service.NewAuthService(repo, config.JWTConfig{
		Secret:     "test-secret-key",
		Expiration: time.Hour,
	})
}

func TestRegister_Success(t *testing.T) {
	repo := NewMockUserRepo()
	svc := newTestAuthService(repo)

	user, err := svc.Register(context.Background(), "alice", "alice@example.com", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID == 0 {
		t.Error("expected user ID to be set")
	}
	if user.Username != "alice" {
		t.Errorf("expected username alice, got %s", user.Username)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	repo := NewMockUserRepo()
	svc := newTestAuthService(repo)

	_, err := svc.Register(context.Background(), "alice", "alice@example.com", "pass1")
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	_, err = svc.Register(context.Background(), "alice2", "alice@example.com", "pass2")
	if err != service.ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	repo := NewMockUserRepo()
	svc := newTestAuthService(repo)

	_, err := svc.Register(context.Background(), "bob", "bob@example.com", "correctpassword")
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = svc.Login(context.Background(), "bob@example.com", "wrongpassword")
	if err != service.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_Success_TokenValid(t *testing.T) {
	repo := NewMockUserRepo()
	svc := newTestAuthService(repo)

	_, err := svc.Register(context.Background(), "carol", "carol@example.com", "mypassword")
	if err != nil {
		t.Fatal(err)
	}

	token, user, err := svc.Login(context.Background(), "carol@example.com", "mypassword")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if user.Email != "carol@example.com" {
		t.Errorf("unexpected user email: %s", user.Email)
	}

	// Validate the token
	userID, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("token validation failed: %v", err)
	}
	if userID != user.ID {
		t.Errorf("expected user ID %d, got %d", user.ID, userID)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	repo := NewMockUserRepo()
	svc := service.NewAuthService(repo, config.JWTConfig{
		Secret:     "test-secret",
		Expiration: -time.Hour, // already expired
	})

	_, err := svc.Register(context.Background(), "dave", "dave@example.com", "pass")
	if err != nil {
		t.Fatal(err)
	}

	token, _, err := svc.Login(context.Background(), "dave@example.com", "pass")
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.ValidateToken(token)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}
