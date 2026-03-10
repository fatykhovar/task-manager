package handler

import (
	"net/http"

	"github.com/fatykhovar/task-manager/internal/repository"
)

type AnalyticsHandler struct {
	taskRepo *repository.TaskRepository
	teamRepo *repository.TeamRepository
}

func NewAnalyticsHandler(taskRepo *repository.TaskRepository, teamRepo *repository.TeamRepository) *AnalyticsHandler {
	return &AnalyticsHandler{taskRepo: taskRepo, teamRepo: teamRepo}
}

// TeamStats godoc
// @Summary Get team statistics
// @Description Retrieve aggregated statistics for teams
// @Tags analytics
// @Produce json
// @Success 200 {object} interface{}
// @Failure 500 {object} map[string]string
// @Router /analytics/team-stats [get]
func (h *AnalyticsHandler) TeamStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.teamRepo.GetTeamStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get team stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// GET /api/v1/analytics/top-users
// Window function: ROW_NUMBER() OVER (PARTITION BY team)
// TopUsersByTeam godoc
// @Summary Get top users by team
// @Description Retrieve top users for each team using window functions
// @Tags analytics
// @Produce json
// @Success 200 {object} interface{}
// @Failure 500 {object} map[string]string
// @Router /analytics/top-users [get]
func (h *AnalyticsHandler) TopUsersByTeam(w http.ResponseWriter, r *http.Request) {
	users, err := h.teamRepo.GetTopUsersByTeam(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get top users")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// GET /api/v1/analytics/integrity-check
// Subquery: tasks where assignee is not a member of task's team
// IntegrityCheck godoc
// @Summary Check data integrity
// @Description Find tasks where the assignee is not a team member
// @Tags analytics
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /analytics/integrity-check [get]
func (h *AnalyticsHandler) IntegrityCheck(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.teamRepo.GetIntegrityViolations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get integrity violations")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"violations": tasks,
		"count":      len(tasks),
	})
}
