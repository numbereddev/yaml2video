package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ondics/yaml2video/src/render"
)

const testOutput = "fake mp4 artifact for handler tests"

func testServer(t *testing.T) *Server {
	t.Helper()
	s, err := NewServer(Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	s.load = func(video, template []byte, media string) (*render.Plan, error) {
		if string(video) != "video yaml" || string(template) != "template yaml" {
			return nil, errors.New("unexpected YAML bytes")
		}
		if media != filepath.Join(s.dataDir, "media") {
			return nil, errors.New("unexpected media directory")
		}
		return &render.Plan{}, nil
	}
	s.render = func(ctx context.Context, p *render.Plan, emit func(render.Progress)) error {
		emit(render.Progress{Stage: "scenes", Percent: 50, Completed: 1, Total: 2})
		return os.WriteFile(p.Output, []byte(testOutput), 0o600)
	}
	return s
}

func renderBody(t *testing.T, id string) []byte {
	t.Helper()
	data, err := json.Marshal(renderRequest{ID: id, Video: []byte("video yaml"), Template: []byte("template yaml")})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func requestRender(t *testing.T, s *Server, id string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(renderBody(t, id))))
	return w
}

func outputRequest(s *Server, method, id string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/output/"+url.PathEscape(id), nil)
	for key, value := range headers {
		r.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func metadataFor(t *testing.T, s *Server, id string) jobMetadata {
	t.Helper()
	m, err := readMetadata(s.jobDir(id))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

type testSSEEvent struct {
	name string
	data map[string]any
}

func parseSSE(t *testing.T, body string) []testSSEEvent {
	t.Helper()
	if !strings.HasSuffix(body, "\n\n") {
		t.Fatalf("unterminated SSE: %q", body)
	}
	var events []testSSEEvent
	for _, frame := range strings.Split(strings.TrimSuffix(body, "\n\n"), "\n\n") {
		lines := strings.Split(frame, "\n")
		if len(lines) != 2 || !strings.HasPrefix(lines[0], "event: ") || !strings.HasPrefix(lines[1], "data: ") {
			t.Fatalf("invalid SSE frame: %q", frame)
		}
		event := testSSEEvent{name: strings.TrimPrefix(lines[0], "event: ")}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[1], "data: ")), &event.data); err != nil {
			t.Fatalf("invalid SSE JSON: %v", err)
		}
		events = append(events, event)
	}
	return events
}

type flushRecorder struct {
	*httptest.ResponseRecorder
	frames []string
}

func (w *flushRecorder) Flush() {
	w.frames = append(w.frames, w.Body.String())
	w.ResponseRecorder.Flush()
}

func TestRenderSSEAndDownload(t *testing.T) {
	s := testServer(t)
	id := "customer/../movie #1"
	w := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(renderBody(t, id))))
	if w.Code != http.StatusOK {
		t.Fatalf("render status = %d: %s", w.Code, w.Body.String())
	}
	for key, want := range map[string]string{"Content-Type": "text/event-stream", "Cache-Control": "no-cache", "X-Accel-Buffering": "no"} {
		if got := w.Header().Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	events := parseSSE(t, w.Body.String())
	if len(events) != 3 {
		t.Fatalf("events = %#v", events)
	}
	if len(w.frames) != len(events) {
		t.Fatalf("flushes = %d, events = %d", len(w.frames), len(events))
	}
	for i, event := range events {
		if event.data["id"] != id {
			t.Errorf("event %d identity = %v", i, event.data["id"])
		}
		if got := len(parseSSE(t, w.frames[i])); got != i+1 {
			t.Errorf("flush %d contains %d events", i, got)
		}
	}
	if events[0].name != "progress" || events[0].data["stage"] != "starting" || events[0].data["percent"] != float64(0) {
		t.Errorf("starting event = %#v", events[0])
	}
	if events[1].name != "progress" || events[1].data["percent"] != float64(50) || events[1].data["completed"] != float64(1) {
		t.Errorf("progress event = %#v", events[1])
	}
	if events[2].name != "complete" || events[2].data["url"] != "/output/"+url.PathEscape(id) {
		t.Errorf("complete event = %#v", events[2])
	}
	m := metadataFor(t, s, id)
	if m.ID != id || !m.Ready || m.FreeableAt != nil {
		t.Fatalf("render metadata = %+v", m)
	}
	before := time.Now()
	downloaded := outputRequest(s, http.MethodGet, id, nil)
	if downloaded.Code != http.StatusOK || downloaded.Body.String() != testOutput {
		t.Fatalf("download = %d %q", downloaded.Code, downloaded.Body.String())
	}
	if downloaded.Header().Get("Content-Type") != "video/mp4" || downloaded.Header().Get("Content-Disposition") != `attachment; filename="output.mp4"` {
		t.Errorf("download headers = %v", downloaded.Header())
	}
	marked := metadataFor(t, s, id)
	if marked.FreeableAt == nil || marked.FreeableAt.Before(before) || marked.FreeableAt.After(time.Now()) {
		t.Fatalf("freeable timestamp = %v", marked.FreeableAt)
	}
	outputRequest(s, http.MethodGet, id, nil)
	again := metadataFor(t, s, id)
	if again.FreeableAt == nil || !again.FreeableAt.Equal(*marked.FreeableAt) {
		t.Fatal("repeat download extended retention")
	}
}

func TestPartialDownloadsDoNotMarkFreeable(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		headers      map[string]string
		status       int
		body         string
	}{
		{"range", http.MethodGet, map[string]string{"Range": "bytes=0-3"}, http.StatusPartialContent, testOutput[:4]},
		{"head", http.MethodHead, nil, http.StatusOK, ""},
		{"not modified", http.MethodGet, map[string]string{"If-Modified-Since": time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)}, http.StatusNotModified, ""},
		{"invalid range", http.MethodGet, map[string]string{"Range": "bytes=9999-"}, http.StatusRequestedRangeNotSatisfiable, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testServer(t)
			requestRender(t, s, "movie")
			w := outputRequest(s, tc.method, "movie", tc.headers)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status != http.StatusRequestedRangeNotSatisfiable && w.Body.String() != tc.body {
				t.Errorf("body = %q, want %q", w.Body.String(), tc.body)
			}
			if metadataFor(t, s, "movie").FreeableAt != nil {
				t.Fatal("non-full download marked freeable")
			}
		})
	}
}

func TestRenderFailuresCleanUp(t *testing.T) {
	for _, stage := range []string{"load", "render"} {
		t.Run(stage, func(t *testing.T) {
			s := testServer(t)
			message := "failed \"quoted\"\nsecond line"
			if stage == "load" {
				s.load = func([]byte, []byte, string) (*render.Plan, error) { return nil, errors.New(message) }
			} else {
				s.render = func(ctx context.Context, p *render.Plan, emit func(render.Progress)) error {
					if err := os.WriteFile(p.Output, []byte("partial"), 0o600); err != nil {
						return err
					}
					emit(render.Progress{Stage: "scenes", Percent: 25})
					return errors.New(message)
				}
			}
			w := requestRender(t, s, "failed")
			if stage == "load" {
				if w.Code != http.StatusBadRequest {
					t.Fatalf("load error status = %d", w.Code)
				}
			} else {
				if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "text/event-stream" {
					t.Fatalf("render error response = %d %v", w.Code, w.Header())
				}
				events := parseSSE(t, w.Body.String())
				last := events[len(events)-1]
				if last.name != "error" || last.data["error"] != message {
					t.Fatalf("error event = %#v", last)
				}
				for _, event := range events {
					if event.name == "complete" {
						t.Fatal("failed render completed")
					}
				}
			}
			if _, err := os.Stat(s.jobDir("failed")); !os.IsNotExist(err) {
				t.Fatalf("failed job retained: %v", err)
			}
			if len(s.active) != 0 {
				t.Fatalf("active jobs = %v", s.active)
			}
			if outputRequest(s, http.MethodGet, "failed", nil).Code != http.StatusNotFound {
				t.Fatal("failed output downloadable")
			}
		})
	}
}

func TestInvalidRenderRequests(t *testing.T) {
	valid := string(renderBody(t, "valid"))
	for _, body := range []string{
		"", "{", "null", "[]", valid + valid,
		`{"id":"x","video":"not base64","template":"eA=="}`,
		`{"id":"x","name":"y","video":"eA==","template":"eA=="}`,
		`{"video":"eA==","template":"eA=="}`,
		`{"id":"x","template":"eA=="}`,
		`{"id":"x","video":"eA=="}`,
		`{"id":"bad\nidentity","video":"eA==","template":"eA=="}`,
		`{"id":"bad\u0000identity","video":"eA==","template":"eA=="}`,
		string(renderBody(t, strings.Repeat("x", 513))),
	} {
		t.Run(body, func(t *testing.T) {
			s := testServer(t)
			s.load = func([]byte, []byte, string) (*render.Plan, error) {
				t.Error("invalid request reached loader")
				return nil, errors.New("unexpected load")
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(body)))
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			entries, err := os.ReadDir(filepath.Join(s.dataDir, "jobs"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid request created jobs: %v %v", entries, err)
			}
		})
	}
	t.Run("oversized", func(t *testing.T) {
		s := testServer(t)
		w := httptest.NewRecorder()
		body := `{"id":"` + strings.Repeat("x", 16<<20) + `"}`
		s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/render", strings.NewReader(body)))
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413", w.Code)
		}
	})
}

func TestNameAliasAndDuplicate(t *testing.T) {
	s := testServer(t)
	body, err := json.Marshal(renderRequest{Name: "alias", Video: []byte("video yaml"), Template: []byte("template yaml")})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(body)))
	if w.Code != http.StatusOK || !metadataFor(t, s, "alias").Ready {
		t.Fatalf("alias render = %d", w.Code)
	}
	duplicate := requestRender(t, s, "alias")
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d", duplicate.Code)
	}
}

func awaitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for handler")
	}
}

func TestActiveRenderDuplicateAndCleanup(t *testing.T) {
	s := testServer(t)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	s.render = func(ctx context.Context, p *render.Plan, emit func(render.Progress)) error {
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		return os.WriteFile(p.Output, []byte(testOutput), 0o600)
	}
	body := renderBody(t, "active")
	w := httptest.NewRecorder()
	go func() {
		defer close(done)
		s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(body)))
	}()
	awaitSignal(t, started)
	if got := requestRender(t, s, "active").Code; got != http.StatusConflict {
		t.Errorf("active duplicate = %d", got)
	}
	if got := outputRequest(s, http.MethodGet, "active", nil).Code; got != http.StatusNotFound {
		t.Errorf("unfinished download = %d", got)
	}
	if err := s.cleanup(time.Now().Add(8 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.jobDir("active")); err != nil {
		t.Fatal("cleanup removed active render:", err)
	}
	unblock()
	awaitSignal(t, done)
	if !metadataFor(t, s, "active").Ready {
		t.Fatal("render did not finish")
	}
	if err := s.cleanup(time.Now().Add(8 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.jobDir("active")); !os.IsNotExist(err) {
		t.Fatalf("inactive expired render retained: %v", err)
	}
}

type failingStream struct{ *httptest.ResponseRecorder }

func (w *failingStream) FlushError() error { return io.ErrClosedPipe }

func TestBrokenStreamCancelsBeforeCleanup(t *testing.T) {
	s := testServer(t)
	stopped := make(chan struct{})
	s.render = func(ctx context.Context, p *render.Plan, emit func(render.Progress)) error {
		defer close(stopped)
		<-ctx.Done()
		if _, err := os.Stat(filepath.Dir(p.Output)); err != nil {
			t.Errorf("job removed before renderer stopped: %v", err)
			return err
		}
		return ctx.Err()
	}
	w := &failingStream{httptest.NewRecorder()}
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(renderBody(t, "broken"))))
	awaitSignal(t, stopped)
	if _, err := os.Stat(s.jobDir("broken")); !os.IsNotExist(err) {
		t.Fatalf("broken stream job retained: %v", err)
	}
	if len(s.active) != 0 {
		t.Fatal("broken stream leaked active reservation")
	}
}

type blockedDownload struct {
	*httptest.ResponseRecorder
	started, release chan struct{}
	once             sync.Once
	fail             bool
}

func (w *blockedDownload) Write(data []byte) (int, error) {
	w.once.Do(func() { close(w.started); <-w.release })
	if w.fail {
		return 0, io.ErrClosedPipe
	}
	return w.ResponseRecorder.Write(data)
}

func TestActiveDownloadCleanupAndWriteFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "write failure"}[fail], func(t *testing.T) {
			s := testServer(t)
			requestRender(t, s, "download")
			w := &blockedDownload{ResponseRecorder: httptest.NewRecorder(), started: make(chan struct{}), release: make(chan struct{}), fail: fail}
			var once sync.Once
			unblock := func() { once.Do(func() { close(w.release) }) }
			defer unblock()
			done := make(chan struct{})
			go func() {
				defer close(done)
				s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/output/download", nil))
			}()
			awaitSignal(t, w.started)
			if err := s.cleanup(time.Now().Add(8 * 24 * time.Hour)); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(s.jobDir("download")); err != nil {
				t.Fatal("cleanup removed active download:", err)
			}
			unblock()
			awaitSignal(t, done)
			m := metadataFor(t, s, "download")
			if (m.FreeableAt != nil) == fail {
				t.Fatalf("failed=%v freeable=%v", fail, m.FreeableAt)
			}
			if err := s.cleanup(time.Now().Add(8 * 24 * time.Hour)); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(s.jobDir("download")); !os.IsNotExist(err) {
				t.Fatalf("download reservation leaked: %v", err)
			}
		})
	}
}

type liveFlushRecorder struct {
	*httptest.ResponseRecorder
	flushed chan string
}

func (w *liveFlushRecorder) Flush() {
	w.ResponseRecorder.Flush()
	w.flushed <- w.Body.String()
}

func TestProgressFlushedWhileRendererRunning(t *testing.T) {
	s := testServer(t)
	release, done := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	s.render = func(ctx context.Context, p *render.Plan, emit func(render.Progress)) error {
		emit(render.Progress{Stage: "scenes", Percent: 50})
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		return os.WriteFile(p.Output, []byte(testOutput), 0o600)
	}
	w := &liveFlushRecorder{ResponseRecorder: httptest.NewRecorder(), flushed: make(chan string, 4)}
	body := renderBody(t, "streaming")
	go func() {
		defer close(done)
		s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(body)))
	}()
	for i := 0; i < 2; i++ {
		select {
		case snapshot := <-w.flushed:
			events := parseSSE(t, snapshot)
			if len(events) != i+1 || events[i].name != "progress" {
				t.Fatalf("live events = %#v", events)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("progress was not flushed before render finished")
		}
	}
	select {
	case <-done:
		t.Fatal("handler completed before renderer released")
	default:
	}
	unblock()
	awaitSignal(t, done)
	events := parseSSE(t, w.Body.String())
	if events[len(events)-1].name != "complete" {
		t.Fatal("stream did not complete")
	}
}

type nonStreamingWriter struct{ recorder *httptest.ResponseRecorder }

func (w *nonStreamingWriter) Header() http.Header            { return w.recorder.Header() }
func (w *nonStreamingWriter) WriteHeader(status int)         { w.recorder.WriteHeader(status) }
func (w *nonStreamingWriter) Write(data []byte) (int, error) { return w.recorder.Write(data) }

func TestRenderRequiresStreaming(t *testing.T) {
	s := testServer(t)
	s.load = func([]byte, []byte, string) (*render.Plan, error) {
		t.Error("non-streaming request reached loader")
		return nil, errors.New("unexpected load")
	}
	w := &nonStreamingWriter{recorder: httptest.NewRecorder()}
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(renderBody(t, "no-stream"))))
	if w.recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.recorder.Code)
	}
	if _, err := os.Stat(s.jobDir("no-stream")); !os.IsNotExist(err) {
		t.Fatalf("non-streaming request reserved job: %v", err)
	}
}

func TestCanceledDownloadDoesNotMarkFreeable(t *testing.T) {
	s := testServer(t)
	requestRender(t, s, "canceled")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodGet, "/output/canceled", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if metadataFor(t, s, "canceled").FreeableAt != nil {
		t.Fatal("canceled download marked freeable")
	}
	if len(s.active) != 0 {
		t.Fatal("canceled download leaked reservation")
	}
}

func TestConcurrentDownloadsAndCleanup(t *testing.T) {
	s := testServer(t)
	requestRender(t, s, "concurrent")
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := outputRequest(s, http.MethodGet, "concurrent", nil)
			if w.Code != http.StatusOK || w.Body.String() != testOutput {
				t.Errorf("concurrent download = %d %q", w.Code, w.Body.String())
			}
			if err := s.cleanup(time.Now()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if metadataFor(t, s, "concurrent").FreeableAt == nil {
		t.Fatal("concurrent downloads did not persist freeable timestamp")
	}
	if len(s.active) != 0 {
		t.Fatalf("active jobs = %v", s.active)
	}
}
