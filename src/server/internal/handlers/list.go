package handlers

import (
	"net/http"

	"github.com/ondics/yaml2video/src/server/internal/api"
)

func (h *Handler) ListRenders(w http.ResponseWriter, r *http.Request) {
	renders, err := h.Controller.ListRenders()
	if err != nil {
		api.JSON(w, http.StatusInternalServerError, api.Map{"error": "cannot list renders"})
		return
	}

	items := make([]api.Map, 0, len(renders))
	for _, render := range renders {
		items = append(items, api.Map{
			"id":         render.ID,
			"created_at": render.CreatedAt,
			"expires_at": render.ExpiresAt(),
			"url":        outputURL(render.ID),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	api.JSON(w, http.StatusOK, items)
}
