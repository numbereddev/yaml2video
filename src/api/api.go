package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ondics/yaml2video/src/render"
)

// NewRouter is retained for embedding. For explicit startup errors and cleanup
// lifecycle control, use NewServer and Server.Handler instead.
func NewRouter() http.Handler {
	s, err := NewServer(ConfigFromEnv())
	if err != nil {
		panic(err)
	}
	go s.RunCleanup(context.Background())
	return s.Handler()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /render", s.handleRender)
	mux.HandleFunc("GET /output/{file_id}", s.handleOutput)
	return Chain(mux, Recover)
}

type renderRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Video    []byte `json:"video"`
	Template []byte `json:"template"`
}

func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	var body renderRequest
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		status := http.StatusBadRequest
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			status = http.StatusRequestEntityTooLarge
		}
		http.Error(w, "invalid render request: "+err.Error(), status)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "request must contain one JSON object", http.StatusBadRequest)
		return
	}

	id := body.ID
	if id == "" {
		id = body.Name
	}
	if id == "" || len(id) > 512 || !utf8.ValidString(id) || strings.ContainsAny(id, "\x00\r\n") || (body.ID != "" && body.Name != "" && body.ID != body.Name) || len(body.Video) == 0 || len(body.Template) == 0 {
		http.Error(w, "provide a name or id (max 512 bytes) and base64 video and template YAML; name and id must match if both are supplied", http.StatusBadRequest)
		return
	}

	controller := http.NewResponseController(w)

	// Verify streaming support before reserving a job or starting FFmpeg.
	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	dir := s.jobDir(id)
	key := jobKey(id)
	s.mu.Lock()
	if s.active[key] > 0 {
		s.mu.Unlock()
		http.Error(w, "render identity already exists", http.StatusConflict)
		return
	}
	if _, err := os.Stat(dir); err == nil {
		metadata, readErr := readMetadata(dir)
		if readErr != nil || !expired(metadata, time.Now()) {
			s.mu.Unlock()
			http.Error(w, "render identity already exists", http.StatusConflict)
			return
		}
		if err := os.RemoveAll(dir); err != nil {
			s.mu.Unlock()
			http.Error(w, "cannot remove expired render", 500)
			return
		}
	} else if !os.IsNotExist(err) {
		s.mu.Unlock()
		http.Error(w, "cannot inspect render storage", 500)
		return
	}

	if err := os.Mkdir(dir, 0o755); err != nil {
		s.mu.Unlock()
		http.Error(w, "cannot create render storage", 500)
		return
	}

	metadata := jobMetadata{ID: id, CreatedAt: time.Now().UTC()}
	if err := writeMetadata(dir, metadata); err != nil {
		os.RemoveAll(dir)
		s.mu.Unlock()
		http.Error(w, "cannot save render metadata", 500)
		return
	}

	s.active[key]++
	s.mu.Unlock()
	ready := false
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if !ready {
			if err := os.RemoveAll(dir); err != nil {
				log.Printf("remove failed render: %v", err)
			}
		}
		s.active[key]--
		if s.active[key] == 0 {
			delete(s.active, key)
		}
	}()

	plan, err := s.load(body.Video, body.Template, filepath.Join(s.dataDir, "media"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	plan.WorkDir = filepath.Join(dir, "work")
	plan.Output = filepath.Join(dir, "output.mp4")
	for i := range plan.Scenes {
		plan.Scenes[i].ASSPath = filepath.Join(plan.WorkDir, "ass", fmt.Sprintf("scene-%04d.ass", i))
		plan.Scenes[i].Output = filepath.Join(plan.WorkDir, "scenes", fmt.Sprintf("scene-%04d.mkv", i))
	}

	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	events := make(chan render.Progress, 16)
	result := make(chan error, 1)

	go func() {
		result <- s.render(ctx, plan, func(progress render.Progress) {
			select {
			case events <- progress:
			case <-ctx.Done():
			}
		})
		close(events)
	}()
	// Never remove work files until FFmpeg has stopped, even on a broken stream.
	finished := false
	defer func() {
		cancel()
		if !finished {
			<-result
		}
	}()

	send := func(event string, value any) error {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
			return err
		}
		return controller.Flush()
	}

	if err := send("progress", map[string]any{"id": id, "stage": "starting", "percent": 0}); err != nil {
		return
	}

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := io.WriteString(w, ": heartbeat\n\n"); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		case progress, ok := <-events:
			if ok {
				if err := send("progress", struct {
					ID string `json:"id"`
					render.Progress
				}{id, progress}); err != nil {
					return
				}
				continue
			}

			err := <-result
			finished = true
			if err != nil {
				_ = send("error", map[string]string{"id": id, "error": err.Error()})
				return
			}

			metadata.Ready = true
			if err := writeMetadata(dir, metadata); err != nil {
				_ = send("error", map[string]string{"id": id, "error": "cannot save completed render"})
				return
			}

			ready = true
			_ = send("complete", map[string]string{"id": id, "url": outputURL(id)})
			return
		}
	}
}

func outputURL(id string) string {
	escaped := url.PathEscape(id)
	// Escape dot-only segments so ServeMux does not redirect them as path navigation.
	if id == "." || id == ".." {
		escaped = strings.ReplaceAll(id, ".", "%2E")
	}
	return "/output/" + escaped
}

func (s *Server) handleOutput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("file_id")
	key := jobKey(id)
	dir := s.jobDir(id)
	s.mu.Lock()

	metadata, err := readMetadata(dir)
	if err != nil || metadata.ID != id || !metadata.Ready || expired(metadata, time.Now()) {
		s.mu.Unlock()
		http.NotFound(w, r)
		return
	}

	file, err := os.Open(filepath.Join(dir, "output.mp4"))
	if err != nil {
		s.mu.Unlock()
		http.NotFound(w, r)
		return
	}

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		s.mu.Unlock()
		http.NotFound(w, r)
		return
	}

	s.active[key]++
	s.mu.Unlock()
	defer file.Close()
	defer func() {
		s.mu.Lock()
		s.active[key]--
		if s.active[key] == 0 {
			delete(s.active, key)
		}
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", `attachment; filename="output.mp4"`)
	w.Header().Set("Cache-Control", "no-store")

	tracked := &downloadWriter{ResponseWriter: w}
	http.ServeContent(tracked, r, "output.mp4", info.ModTime(), file)

	// Range responses do not prove that the complete artifact was retrieved.
	if r.Method != http.MethodGet || r.Header.Get("Range") != "" || tracked.status != http.StatusOK || tracked.failed || tracked.bytes != info.Size() || r.Context().Err() != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, err = readMetadata(dir)
	if err == nil && metadata.FreeableAt == nil {
		now := time.Now().UTC()
		metadata.FreeableAt = &now
		if err := writeMetadata(dir, metadata); err != nil {
			log.Printf("mark render freeable: %v", err)
		}
	}
}

type downloadWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
	failed bool
}

func (w *downloadWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *downloadWriter) Write(data []byte) (int, error) {
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
