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

type TaskHandler struct {
	tasks    *service.TaskService
	comments *service.CommentService
}

func NewTaskHandler(tasks *service.TaskService, comments *service.CommentService) *TaskHandler {
	return &TaskHandler{tasks: tasks, comments: comments}
}

// CreateTask godoc
// @Summary Create a new task
// @Description Add a new task to a team
// @Tags tasks
// @Accept json
// @Produce json
// @Param task body model.Task true "Task details"
// @Success 201 {object} model.Task
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /tasks [post]
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var t model.Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if t.Title == "" || t.TeamID == 0 {
		writeError(w, http.StatusBadRequest, "title and team_id are required")
		return
	}
	if t.Priority == "" {
		t.Priority = model.PriorityMedium
	}

	task, err := h.tasks.CreateTask(r.Context(), userID, &t)
	if err == service.ErrNotTeamMember {
		writeError(w, http.StatusForbidden, "not a team member")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

// ListTasks godoc
// @Summary List tasks
// @Description Retrieve tasks based on filters
// @Tags tasks
// @Produce json
// @Param team_id query int false "Team ID"
// @Param status query string false "Task status"
// @Param assignee_id query int false "Assignee ID"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} model.PaginatedResponse
// @Failure 500 {object} map[string]string
// @Router /tasks [get]
func (h *TaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := model.TaskFilter{}

	if v := q.Get("team_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		f.TeamID = &id
	}
	if v := q.Get("status"); v != "" {
		s := model.TaskStatus(v)
		f.Status = &s
	}
	if v := q.Get("assignee_id"); v != "" {
		id, _ := strconv.ParseInt(v, 10, 64)
		f.AssigneeID = &id
	}
	f.Page, _ = strconv.Atoi(q.Get("page"))
	f.PageSize, _ = strconv.Atoi(q.Get("page_size"))

	tasks, total, err := h.tasks.ListTasks(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}

	writeJSON(w, http.StatusOK, model.PaginatedResponse{
		Data:     tasks,
		Total:    total,
		Page:     f.Page,
		PageSize: f.PageSize,
	})
}

// UpdateTask godoc
// @Summary Update a task
// @Description Modify task details
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path int true "Task ID"
// @Param updates body model.Task true "Updated task details"
// @Success 200 {object} model.Task
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /tasks/{id} [put]
func (h *TaskHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	taskID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	var updates model.Task
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	task, err := h.tasks.UpdateTask(r.Context(), userID, taskID, &updates)
	if err != nil {
		switch err {
		case service.ErrTaskNotFound:
			writeError(w, http.StatusNotFound, "task not found")
		case service.ErrUnauthorized:
			writeError(w, http.StatusForbidden, "not authorized")
		default:
			writeError(w, http.StatusInternalServerError, "failed to update task")
		}
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// GetTaskHistory godoc
// @Summary Get task history
// @Description Retrieve the history of a task
// @Tags tasks
// @Produce json
// @Param id path int true "Task ID"
// @Success 200 {object} interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /tasks/{id}/history [get]
func (h *TaskHandler) GetTaskHistory(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	taskID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	history, err := h.tasks.GetHistory(r.Context(), userID, taskID)
	if err != nil {
		switch err {
		case service.ErrTaskNotFound:
			writeError(w, http.StatusNotFound, "task not found")
		case service.ErrUnauthorized:
			writeError(w, http.StatusForbidden, "not authorized")
		default:
			writeError(w, http.StatusInternalServerError, "failed to get history")
		}
		return
	}
	writeJSON(w, http.StatusOK, history)
}

// AddComment godoc
// @Summary Add a comment to a task
// @Description Post a comment on a task
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path int true "Task ID"
// @Param comment body map[string]string true "Comment content"
// @Success 201 {object} interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /tasks/{id}/comments [post]
func (h *TaskHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	taskID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	comment, err := h.comments.AddComment(r.Context(), userID, taskID, req.Content)
	if err == service.ErrCannotComment {
		writeError(w, http.StatusForbidden, "not a team member")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add comment")
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}
