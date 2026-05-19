package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/go-chi/chi/v5"

	"github.com/brandonthis-that/taskOS-server/internal/auth"
	"github.com/brandonthis-that/taskOS-server/internal/models"
	"github.com/brandonthis-that/taskOS-server/internal/store"
)

var usernameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

// API wires HTTP handlers to the store and auth service.
type API struct {
	store *store.Store
	auth  *auth.Service
}

func NewAPI(st *store.Store, authSvc *auth.Service) *API {
	return &API{store: st, auth: authSvc}
}

func (a *API) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "taskOS",
	})
}

type registerRequest struct {
	Username string `json:"username"`
}

type registerResponse struct {
	User   models.User `json:"user"`
	APIKey string      `json:"api_key"`
	Note   string      `json:"note"`
}

func (a *API) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !usernameRE.MatchString(req.Username) {
		writeError(w, http.StatusBadRequest, "invalid_username", "username must be 3-32 chars: letters, numbers, _ or -")
		return
	}

	plain, hash, lookup, err := auth.GenerateAPIKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not create credentials")
		return
	}

	now := time.Now().UTC()
	user, err := a.store.CreateUser(r.Context(), uuid.NewString(), req.Username, lookup, hash, now)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "registration failed")
		return
	}

	writeJSON(w, http.StatusCreated, registerResponse{
		User:   *user,
		APIKey: plain,
		Note:   "Store this API key securely. It is shown only once.",
	})
}

func (a *API) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	user, err := a.store.UserByID(r.Context(), userID)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to load profile")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (a *API) LookupUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	user, err := a.store.UserByUsername(r.Context(), username)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "lookup failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":       user.ID,
		"username": user.Username,
	})
}

func currentUserID(r *http.Request) (string, bool) {
	return auth.UserIDFromContext(r.Context())
}
