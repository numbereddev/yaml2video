package handlers

import (
	"net/http"

	"github.com/ondics/yaml2video/src/server/internal/api"
)

func (h *Handler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	artifact, err := h.Controller.OpenOutput(r.PathValue("id"))
	if err != nil {
		api.JSON(w, http.StatusNotFound, api.Map{"error": "output not found"})
		return
	}
	defer artifact.Close()

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", `attachment; filename="output.mp4"`)
	w.Header().Set("Cache-Control", "no-store")
	tracked := &DownloadWriter{ResponseWriter: w}
	http.ServeContent(tracked, r, "output.mp4", artifact.Info.ModTime(), artifact.File)

	// Partial or failed downloads must not start the shorter retention period.
	failed := r.Method != http.MethodGet ||
		r.Header.Get("Range") != "" ||
		tracked.status != http.StatusOK ||
		tracked.failed ||
		tracked.bytes != artifact.Info.Size() ||
		r.Context().Err() != nil
	if failed {
		return
	}
	artifact.MarkFreeable()
}

type DownloadWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
	failed bool
}

func (w *DownloadWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *DownloadWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(data)
	w.bytes += int64(n)
	if err != nil || n != len(data) {
		w.failed = true
	}
	return n, err
}
