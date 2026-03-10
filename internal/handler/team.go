package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/fatykhovar/task-manager/internal/middleware"
	"github.com/fatykhovar/task-manager/internal/model"
	"github.com/fatykhovar/task-manager/internal/service"
	"github.com/go-chi/chi/v5"
)

type TeamHandler struct {
	teams *service.TeamService
	email *service.EmailService
}

func NewTeamHandler(teams *service.TeamService, email *service.EmailService) *TeamHandler {
	return &TeamHandler{teams: teams, email: email}
}

// CreateTeam godoc
// @Summary Create a new team
// @Description Add a new team with a description
// @Tags teams
// @Accept json
// @Produce json
// @Param team body map[string]string true "Team details"
// @Success 201 {object} model.Team
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /teams [post]
func (h *TeamHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	team, err := h.teams.CreateTeam(r.Context(), userID, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create team")
		return
	}
	writeJSON(w, http.StatusCreated, team)
}

// ListTeams godoc
// @Summary List teams
// @Description Retrieve all teams for the user
// @Tags teams
// @Produce json
// @Success 200 {object} []model.Team
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /teams [get]
func (h *TeamHandler) ListTeams(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	teams, err := h.teams.ListTeams(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list teams")
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

// InviteUser godoc
// @Summary Invite a user to a team
// @Description Send an invitation to a user to join a team
// @Tags teams
// @Accept json
// @Produce json
// @Param id path int true "Team ID"
// @Param invitation body map[string]string true "Invitation details"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /teams/{id}/invite [post]
func (h *TeamHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	teamID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid team id")
		return
	}

	var req struct {
		Email string         `json:"email"`
		Role  model.TeamRole `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Role == "" {
		req.Role = model.RoleMember
	}

	if err := h.teams.InviteUser(r.Context(), userID, teamID, req.Email, req.Role); err != nil {
		switch err {
		case service.ErrNotTeamMember:
			writeError(w, http.StatusForbidden, "not a team member")
		case service.ErrInsufficientRole:
			writeError(w, http.StatusForbidden, "insufficient permissions")
		case service.ErrUserNotFound:
			writeError(w, http.StatusNotFound, "user not found")
		default:
			writeError(w, http.StatusInternalServerError, "failed to invite user")
		}
		return
	}

	// Send email (best-effort, don't fail request if circuit is open)
	h.email.SendInvitation(r.Context(), req.Email, "")

	writeJSON(w, http.StatusOK, map[string]string{"status": "invited"})
}
