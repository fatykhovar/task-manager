package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fatykhovar/task-manager/internal/config"
	"go.uber.org/zap"
)

type circuitState int

const (
	stateClosed   circuitState = iota // Normal operation
	stateOpen                         // Failing, reject calls
	stateHalfOpen                     // Testing recovery
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu           sync.Mutex
	state        circuitState
	failures     int
	maxFailures  int
	resetTimeout time.Duration
	lastFailure  time.Time
}

func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
	}
}

func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateOpen:
		if time.Since(cb.lastFailure) >= cb.resetTimeout {
			cb.state = stateHalfOpen
			return nil
		}
		return ErrCircuitOpen
	case stateHalfOpen, stateClosed:
		return nil
	}
	return nil
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = stateClosed
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.maxFailures {
		cb.state = stateOpen
	}
}

// EmailService sends invitation emails via external mock service
type EmailService struct {
	serviceURL string
	client     *http.Client
	cb         *CircuitBreaker
	logger     *zap.Logger
}

func NewEmailServiceWithCircuitBreaker(cfg config.EmailConfig, logger *zap.Logger) *EmailService {
	return &EmailService{
		serviceURL: cfg.ServiceURL,
		client:     &http.Client{Timeout: cfg.Timeout},
		cb:         NewCircuitBreaker(cfg.MaxFailures, cfg.ResetTimeout),
		logger:     logger,
	}
}

func (s *EmailService) SendInvitation(ctx context.Context, toEmail, teamName string) error {
	if err := s.cb.Allow(); err != nil {
		s.logger.Warn("circuit breaker open, skipping email", zap.String("email", toEmail))
		return err
	}

	payload := map[string]string{
		"to":      toEmail,
		"subject": fmt.Sprintf("You've been invited to team: %s", teamName),
		"body":    fmt.Sprintf("You have been invited to join the team '%s' on Task Manager.", teamName),
	}

	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.serviceURL+"/send", bytes.NewReader(data))
	if err != nil {
		s.cb.RecordFailure()
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.cb.RecordFailure()
		s.logger.Error("email service error", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		s.cb.RecordFailure()
		return fmt.Errorf("email service returned %d", resp.StatusCode)
	}

	s.cb.RecordSuccess()
	return nil
}
