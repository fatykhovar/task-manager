package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/fatykhovar/task-manager/internal/model"
	"github.com/fatykhovar/task-manager/internal/repository"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgC, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		fmt.Printf("failed to start postgres container: %v\n", err)
		os.Exit(1)
	}
	defer pgC.Terminate(ctx)

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Printf("failed to get connection string: %v\n", err)
		os.Exit(1)
	}

	for i := 0; i < 10; i++ {
		db, err := sql.Open("postgres", connStr)
		if err == nil {
			if pingErr := db.Ping(); pingErr == nil {
				testDB = db
				break
			}
			db.Close()
		}
		time.Sleep(time.Second)
	}
	if testDB == nil {
		fmt.Println("could not connect to test database")
		os.Exit(1)
	}

	if err := runMigrations(testDB); err != nil {
		fmt.Printf("migration failed: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func runMigrations(db *sql.DB) error {
	schema, err := os.ReadFile("../../migrations/001_initial_schema.sql")
	if err != nil {
		return err
	}
	_, err = db.Exec(string(schema))
	return err
}

// --- Tests ---

func TestUserRepository_CreateAndFind(t *testing.T) {
	repo := repository.NewUserRepository(testDB)
	ctx := context.Background()

	u := &model.User{
		Username:     fmt.Sprintf("integtest_%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("integtest_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "$2a$10$fakehash",
	}

	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if u.ID == 0 {
		t.Error("expected ID to be set after creation")
	}

	found, err := repo.FindByEmail(ctx, u.Email)
	if err != nil {
		t.Fatalf("find by email: %v", err)
	}
	if found == nil {
		t.Fatal("expected user, got nil")
	}
	if found.Username != u.Username {
		t.Errorf("expected username %s, got %s", u.Username, found.Username)
	}
}

func TestTeamRepository_CreateAndList(t *testing.T) {
	userRepo := repository.NewUserRepository(testDB)
	teamRepo := repository.NewTeamRepository(testDB)
	ctx := context.Background()

	owner := &model.User{
		Username:     fmt.Sprintf("teamowner_%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("owner_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "$2a$10$fakehash",
	}
	if err := userRepo.Create(ctx, owner); err != nil {
		t.Fatal(err)
	}

	team := &model.Team{
		Name:        "Integration Team",
		Description: "test team",
		CreatedBy:   owner.ID,
	}
	if err := teamRepo.Create(ctx, team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	if team.ID == 0 {
		t.Error("expected team ID to be set")
	}

	teams, err := teamRepo.ListByUserID(ctx, owner.ID)
	if err != nil {
		t.Fatalf("list teams: %v", err)
	}
	if len(teams) == 0 {
		t.Error("expected at least one team")
	}
}

func TestTaskRepository_CreateListUpdateHistory(t *testing.T) {
	userRepo := repository.NewUserRepository(testDB)
	teamRepo := repository.NewTeamRepository(testDB)
	taskRepo := repository.NewTaskRepository(testDB)
	ctx := context.Background()

	creator := &model.User{
		Username:     fmt.Sprintf("taskuser_%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("taskuser_%d@test.com", time.Now().UnixNano()),
		PasswordHash: "x",
	}
	if err := userRepo.Create(ctx, creator); err != nil {
		t.Fatal(err)
	}
	team := &model.Team{Name: "Task Team", CreatedBy: creator.ID}
	if err := teamRepo.Create(ctx, team); err != nil {
		t.Fatal(err)
	}

	task := &model.Task{
		Title:     "Test Task",
		Status:    model.StatusTodo,
		Priority:  model.PriorityHigh,
		TeamID:    team.ID,
		CreatedBy: creator.ID,
	}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.ID == 0 {
		t.Fatal("expected task ID to be set")
	}

	// List with filter
	tasks, total, err := taskRepo.List(ctx, model.TaskFilter{TeamID: &team.ID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if total == 0 || len(tasks) == 0 {
		t.Error("expected at least one task")
	}

	// Update with history
	task.Status = model.StatusDone
	changes := []model.TaskHistory{{
		TaskID:    task.ID,
		ChangedBy: creator.ID,
		FieldName: "status",
		OldValue:  "todo",
		NewValue:  "done",
	}}
	if err := taskRepo.Update(ctx, task, changes); err != nil {
		t.Fatalf("update task: %v", err)
	}

	history, err := taskRepo.GetHistory(ctx, task.ID)
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(history) == 0 {
		t.Error("expected history entry")
	}
	if history[0].FieldName != "status" {
		t.Errorf("expected field 'status', got %s", history[0].FieldName)
	}
}

func TestTeamRepository_AddMemberUpsert(t *testing.T) {
	userRepo := repository.NewUserRepository(testDB)
	teamRepo := repository.NewTeamRepository(testDB)
	ctx := context.Background()

	owner := &model.User{
		Username:     fmt.Sprintf("upsertowner_%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("upsertowner_%d@test.com", time.Now().UnixNano()),
		PasswordHash: "x",
	}
	member := &model.User{
		Username:     fmt.Sprintf("upsertmember_%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("upsertmember_%d@test.com", time.Now().UnixNano()),
		PasswordHash: "x",
	}
	userRepo.Create(ctx, owner)
	userRepo.Create(ctx, member)

	team := &model.Team{Name: "Upsert Team", CreatedBy: owner.ID}
	if err := teamRepo.Create(ctx, team); err != nil {
		t.Fatal(err)
	}

	// Add member
	if err := teamRepo.AddMember(ctx, &model.TeamMember{UserID: member.ID, TeamID: team.ID, Role: model.RoleMember}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	// Upsert to admin
	if err := teamRepo.AddMember(ctx, &model.TeamMember{UserID: member.ID, TeamID: team.ID, Role: model.RoleAdmin}); err != nil {
		t.Fatalf("upsert member: %v", err)
	}

	got, err := teamRepo.GetMember(ctx, team.ID, member.ID)
	if err != nil || got == nil {
		t.Fatalf("get member: %v", err)
	}
	if got.Role != model.RoleAdmin {
		t.Errorf("expected role admin, got %s", got.Role)
	}
}
