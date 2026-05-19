package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/brandonthis-that/taskOS-server/internal/models"
)

type contactRequest struct {
	Username         string `json:"username"`
	CanAssignTasks   bool   `json:"can_assign_tasks"`
	CanPingReminders bool   `json:"can_ping_reminders"`
}

func (a *API) ListContacts(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	contacts, err := a.store.ListTrustedContacts(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to list contacts")
		return
	}
	if contacts == nil {
		contacts = []models.TrustedContact{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"contacts": contacts})
}

func (a *API) AddContact(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	var req contactRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "username is required")
		return
	}
	if !req.CanAssignTasks && !req.CanPingReminders {
		writeError(w, http.StatusBadRequest, "invalid_input", "grant at least one permission")
		return
	}

	contactUser, err := a.store.UserByUsername(r.Context(), req.Username)
	if err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "lookup failed")
		return
	}
	if contactUser.ID == userID {
		writeError(w, http.StatusBadRequest, "invalid_input", "cannot add yourself as a contact")
		return
	}

	now := time.Now().UTC()
	c := &models.TrustedContact{
		OwnerID:          userID,
		ContactUserID:    contactUser.ID,
		ContactUsername:  contactUser.Username,
		CanAssignTasks:   req.CanAssignTasks,
		CanPingReminders: req.CanPingReminders,
		CreatedAt:        now,
	}
	if err := a.store.UpsertTrustedContact(r.Context(), c); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "failed to save contact")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (a *API) RemoveContact(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	if err := a.store.DeleteTrustedContact(r.Context(), userID, chi.URLParam(r, "contactUserID")); err != nil {
		if mapStoreError(w, err) {
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "failed to remove contact")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
