package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatykhovar/task-manager/internal/cache"
	"github.com/fatykhovar/task-manager/internal/config"
	"github.com/fatykhovar/task-manager/internal/handler"
	"github.com/fatykhovar/task-manager/internal/middleware"
	"github.com/fatykhovar/task-manager/internal/repository"
	"github.com/fatykhovar/task-manager/internal/service"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	_ "github.com/swaggo/http-swagger" // http-swagger middleware
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logger.Sync()

	db, err := repository.NewPostgres(cfg.Database)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer db.Close()

	redisClient, err := cache.NewRedis(cfg.Redis)
	if err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer redisClient.Close()

	userRepo := repository.NewUserRepository(db)
	teamRepo := repository.NewTeamRepository(db)
	taskRepo := repository.NewTaskRepository(db)
	commentRepo := repository.NewCommentRepository(db)

	taskCache := cache.NewTaskCache(redisClient, cfg.Cache.TaskTTL)

	authSvc := service.NewAuthService(userRepo, cfg.JWT)
	teamSvc := service.NewTeamService(teamRepo, userRepo)
	taskSvc := service.NewTaskService(taskRepo, teamRepo, taskCache, logger)
	commentSvc := service.NewCommentService(commentRepo, taskRepo)
	emailSvc := service.NewEmailServiceWithCircuitBreaker(cfg.Email, logger)

	authHandler := handler.NewAuthHandler(authSvc)
	teamHandler := handler.NewTeamHandler(teamSvc, emailSvc)
	taskHandler := handler.NewTaskHandler(taskSvc, commentSvc)
	analyticsHandler := handler.NewAnalyticsHandler(taskRepo, teamRepo)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.Metrics())
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.RateLimit(cfg.RateLimit))

	r.Handle("/metrics", promhttp.Handler())
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(cfg.JWT))

			r.Post("/teams", teamHandler.CreateTeam)
			r.Get("/teams", teamHandler.ListTeams)
			r.Post("/teams/{id}/invite", teamHandler.InviteUser)

			r.Post("/tasks", taskHandler.CreateTask)
			r.Get("/tasks", taskHandler.ListTasks)
			r.Put("/tasks/{id}", taskHandler.UpdateTask)
			r.Get("/tasks/{id}/history", taskHandler.GetTaskHistory)
			r.Post("/tasks/{id}/comments", taskHandler.AddComment)

			r.Get("/analytics/team-stats", analyticsHandler.TeamStats)
			r.Get("/analytics/top-users", analyticsHandler.TopUsersByTeam)
			r.Get("/analytics/integrity-check", analyticsHandler.IntegrityCheck)
		})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// graceful shutdown
	done := make(chan struct{})
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		logger.Info("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("server forced to shutdown", zap.Error(err))
		}
		close(done)
	}()

	logger.Info("server starting", zap.Int("port", cfg.Server.Port))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("server error", zap.Error(err))
	}

	<-done
	logger.Info("server stopped")
}
