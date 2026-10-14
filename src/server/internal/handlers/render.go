package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/ondics/yaml2video/src/render"
	"github.com/ondics/yaml2video/src/server/internal/api"
	"github.com/ondics/yaml2video/src/server/internal/controller"
)

type RenderRequest struct {
	ID string `json:"id"`

	Video    []byte `json:"video"`
	Template []byte `json:"template"`
}

func (h *Handler) Render(w http.ResponseWriter, r *http.Request) {
	var body RenderRequest
	if err := api.BindJSON(w, r, &body); err != nil {
		status := http.StatusBadRequest
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			status = http.StatusRequestEntityTooLarge
		}

		api.JSON(w, status, api.Map{
			"error": "invalid render request: " + err.Error(),
		})
		return
	}

	id := body.ID
	idValid := id == "" ||
		len(id) > 512 ||
		!utf8.ValidString(id) ||
		strings.ContainsAny(id, "\x00\r\n") ||
		len(body.Video) == 0 ||
		len(body.Template) == 0
	if idValid {
		api.JSON(w, http.StatusBadRequest, api.Map{
			"error": "provide an id (max 512 bytes) and base64 video and template YAML",
		})
		return
	}

	api.ServeSSE(w, r, func(stream *api.SSE) {
		job, err := h.Controller.PrepareRender(id, body.Video, body.Template)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, controller.ErrIdentityExists) {
				status = http.StatusConflict
			} else if _, ok := errors.AsType[controller.InvalidPlanError](err); ok {
				status = http.StatusBadRequest
			}

			api.JSON(w, status, api.Map{"error": err.Error()})
			return
		}
		defer job.Close()

		if err := stream.Send("progress", api.Map{
			"id":      id,
			"stage":   "starting",
			"percent": 0,
		}); err != nil {
			return
		}

		if err := job.Run(r.Context(), func(progress render.Progress) {
			// Send cancels the request on failure, which stops the renderer.
			stream.Send("progress", struct {
				ID string `json:"id"`
				render.Progress
			}{id, progress})
		}); err != nil {
			stream.Send("error", api.Map{
				"id":    id,
				"error": err.Error(),
			})
			return
		}

		stream.Send("complete", api.Map{
			"id":  id,
			"url": outputURL(id),
		})
	})
}

func outputURL(id string) string {
	escaped := url.PathEscape(id)
	// Escape dot-only segments so ServeMux does not redirect them as path navigation.
	if id == "." || id == ".." {
		escaped = strings.ReplaceAll(id, ".", "%2E")
	}
	return "/renders/" + escaped
}
