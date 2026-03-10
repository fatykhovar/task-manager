package unit_test

import (
	"testing"
	"time"

	"github.com/fatykhovar/task-manager/internal/service"
)

func TestCircuitBreaker_ClosedAllowsRequests(t *testing.T) {
	cb := service.NewCircuitBreaker(3, 10*time.Second)
	if err := cb.Allow(); err != nil {
		t.Errorf("expected no error in closed state, got %v", err)
	}
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	cb := service.NewCircuitBreaker(3, 10*time.Second)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if err := cb.Allow(); err != service.ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_ResetsAfterTimeout(t *testing.T) {
	cb := service.NewCircuitBreaker(2, 50*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()

	if err := cb.Allow(); err != service.ErrCircuitOpen {
		t.Fatalf("expected open circuit, got %v", err)
	}

	time.Sleep(60 * time.Millisecond)

	if err := cb.Allow(); err != nil {
		t.Errorf("expected circuit to allow after reset timeout, got %v", err)
	}
}

func TestCircuitBreaker_SuccessResetFailures(t *testing.T) {
	cb := service.NewCircuitBreaker(5, 10*time.Second)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordFailure()

	// Only 2 failures since last success — circuit should be closed
	if err := cb.Allow(); err != nil {
		t.Errorf("expected circuit closed after success reset, got %v", err)
	}
}
