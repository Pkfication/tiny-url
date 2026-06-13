package api

import (
	"encoding/json"
	"net/http"

	"kgs/internal/core"
)

type Handler struct {
	keyService *core.KeyService
}

func NewHandler(ks *core.KeyService) *Handler {
	return &Handler{
		keyService: ks,
	}
}

func (h *Handler) GetKey(w http.ResponseWriter, r *http.Request) {
	key, err := h.keyService.GetNextKey()
	if err != nil {
		http.Error(w, "Failed to generate key", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"key": key,
	})
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/key", h.GetKey)
}
