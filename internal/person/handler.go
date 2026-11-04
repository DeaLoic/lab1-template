package person

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Handler — HTTP-обработчики REST API.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register подключает маршруты API к mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/persons", h.list)
	mux.HandleFunc("POST /api/v1/persons", h.create)
	mux.HandleFunc("GET /api/v1/persons/{id}", h.get)
	mux.HandleFunc("PATCH /api/v1/persons/{id}", h.patch)
	mux.HandleFunc("DELETE /api/v1/persons/{id}", h.remove)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	people, err := h.service.List(r.Context())
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, people)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeRequest(w, r)
	if !ok {
		return
	}
	id, err := h.service.Create(r.Context(), req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	// 201 Created, пустое тело, Location на созданную запись.
	w.Header().Set("Location", fmt.Sprintf("/api/v1/persons/%d", id))
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	resp, err := h.service.Get(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) patch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	req, ok := decodeRequest(w, r)
	if !ok {
		return
	}
	resp, err := h.service.Patch(r.Context(), id, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		handleServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeRequest читает JSON-тело запроса.
func decodeRequest(w http.ResponseWriter, r *http.Request) (Request, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(body) == 0 {
		writeValidation(w, "invalid request body", map[string]string{"body": "must be valid JSON"})
		return Request{}, false
	}
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		writeValidation(w, "invalid request body", map[string]string{"body": err.Error()})
		return Request{}, false
	}
	return req, true
}

// parseID разбирает {id} из пути. Не число → 404.
func parseID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		writeError(w, http.StatusNotFound, "person not found: invalid id "+raw)
		return 0, false
	}
	return int32(id), true
}

func handleServiceError(w http.ResponseWriter, err error) {
	var verr *ValidationError
	switch {
	case errors.As(err, &verr):
		writeValidation(w, "validation failed", verr.Fields)
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "person not found")
	default:
		writeInternal(w, err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Message: message})
}

func writeValidation(w http.ResponseWriter, message string, fields map[string]string) {
	writeJSON(w, http.StatusBadRequest, ValidationErrorResponse{Message: message, Errors: fields})
}

func writeInternal(w http.ResponseWriter, _ error) {
	writeError(w, http.StatusInternalServerError, "internal error")
}
