package handlers

import (
	"encoding/base64"
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

	Video    string `json:"video"`
	Template string `json:"template"`
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

	// Base64 decode the video and template
	video, err := base64.StdEncoding.DecodeString(body.Video)
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Map{"error": "video must be a base64-encoded YAML string"})
		return
	}
	template, err := base64.StdEncoding.DecodeString(body.Template)
	if err != nil {
		api.JSON(w, http.StatusBadRequest, api.Map{"error": "template must be a base64-encoded YAML string"})
		return
	}

	id := body.ID
	idValid := id == "" ||
		len(id) > 512 ||
		!utf8.ValidString(id) ||
		strings.ContainsAny(id, "\x00\r\n") ||
		len(video) == 0 ||
		len(template) == 0
	if idValid {
		api.JSON(w, http.StatusBadRequest, api.Map{
			"error": "provide an id (max 512 bytes) and base64 video and template YAML",
		})
		return
	}

	api.ServeSSE(w, r, func(stream *api.SSE) {
		job, err := h.Controller.PrepareRender(id, video, template)
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

		if err := job.Run(stream.Context(), func(progress render.Progress) {
			// Send cancels the stream context on failure, which stops the renderer.
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
