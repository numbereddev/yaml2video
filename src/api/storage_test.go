package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func seedJob(t *testing.T, s *Server, m jobMetadata) {
	t.Helper()
	dir := s.jobDir(m.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeMetadata(dir, m); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "output.mp4"), []byte(testOutput), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestJobIdentityPathSafety(t *testing.T) {
	s := testServer(t)
	seen := make(map[string]string)
	for _, id := range []string{".", "..", "../escape", "/absolute/path", "nested/movie", `..\windows\path`, "hash#fragment", "name%2Fslash", "普通话", "Movie", "movie"} {
		sum := sha256.Sum256([]byte(id))
		want := hex.EncodeToString(sum[:])
		key := jobKey(id)
		if key != want || len(key) != 64 {
			t.Errorf("jobKey(%q) = %q, want %q", id, key, want)
		}
		if previous, ok := seen[key]; ok {
			t.Errorf("identities %q and %q share directory", previous, id)
		}
		seen[key] = id
		dir := s.jobDir(id)
		if filepath.Dir(dir) != filepath.Join(s.dataDir, "jobs") || filepath.Base(dir) != want {
			t.Errorf("unsafe jobDir(%q) = %q", id, dir)
		}
		w := requestRender(t, s, id)
		if w.Code != http.StatusOK {
			t.Fatalf("render %q = %d: %s", id, w.Code, w.Body.String())
		}
		if metadataFor(t, s, id).ID != id {
			t.Errorf("identity %q changed in metadata", id)
		}
		downloaded := httptest.NewRecorder()
		s.Handler().ServeHTTP(downloaded, httptest.NewRequest(http.MethodGet, outputURL(id), nil))
		if downloaded.Code != http.StatusOK || downloaded.Body.String() != testOutput {
			t.Errorf("escaped download %q = %d %q", id, downloaded.Code, downloaded.Body.String())
		}
	}
}

func TestExpiredBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	justDownloaded := now
	for _, tc := range []struct {
		name          string
		age           time.Duration
		downloadedAge *time.Duration
		want          bool
	}{
		{name: "new"},
		{name: "before seven days", age: 7*24*time.Hour - time.Nanosecond},
		{name: "at seven days", age: 7 * 24 * time.Hour, want: true},
		{name: "after seven days", age: 7*24*time.Hour + time.Nanosecond, want: true},
		{name: "before downloaded TTL", age: time.Hour, downloadedAge: durationPointer(59*time.Minute - time.Nanosecond)},
		{name: "at downloaded TTL", age: time.Hour, downloadedAge: durationPointer(59 * time.Minute), want: true},
		{name: "after downloaded TTL", age: time.Hour, downloadedAge: durationPointer(59*time.Minute + time.Nanosecond), want: true},
		{name: "download cannot extend seven days", age: 7 * 24 * time.Hour, downloadedAge: durationPointer(0), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := jobMetadata{CreatedAt: now.Add(-tc.age)}
			if tc.downloadedAge != nil {
				downloaded := justDownloaded.Add(-*tc.downloadedAge)
				m.FreeableAt = &downloaded
			}
			if got := expired(m, now); got != tc.want {
				t.Errorf("expired = %v, want %v: %+v", got, tc.want, m)
			}
		})
	}
}

func durationPointer(d time.Duration) *time.Duration { return &d }

func TestCleanupTTLAndActiveExclusions(t *testing.T) {
	s := testServer(t)
	now := time.Now().UTC()
	downloadedBefore := now.Add(-59*time.Minute + time.Nanosecond)
	downloadedAt := now.Add(-59 * time.Minute)
	cases := []struct {
		m       jobMetadata
		active  bool
		removed bool
	}{
		{m: jobMetadata{ID: "recent", CreatedAt: now, Ready: true}},
		{m: jobMetadata{ID: "seven days minus one", CreatedAt: now.Add(-7*24*time.Hour + time.Nanosecond), Ready: true}},
		{m: jobMetadata{ID: "seven days", CreatedAt: now.Add(-7 * 24 * time.Hour), Ready: true}, removed: true},
		{m: jobMetadata{ID: "downloaded before TTL", CreatedAt: now.Add(-time.Hour), FreeableAt: &downloadedBefore, Ready: true}},
		{m: jobMetadata{ID: "downloaded at TTL", CreatedAt: now.Add(-time.Hour), FreeableAt: &downloadedAt, Ready: true}, removed: true},
		{m: jobMetadata{ID: "active render", CreatedAt: now.Add(-8 * 24 * time.Hour)}, active: true},
		{m: jobMetadata{ID: "active download", CreatedAt: now.Add(-time.Hour), FreeableAt: &downloadedAt, Ready: true}, active: true},
	}
	for _, tc := range cases {
		seedJob(t, s, tc.m)
		if tc.active {
			s.active[jobKey(tc.m.ID)] = 1
		}
	}
	if err := s.cleanup(now); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		_, err := os.Stat(s.jobDir(tc.m.ID))
		if tc.removed {
			if !os.IsNotExist(err) {
				t.Errorf("%q should be removed: %v", tc.m.ID, err)
			}
		} else if err != nil {
			t.Errorf("%q should survive: %v", tc.m.ID, err)
		}
	}
	s.mu.Lock()
	clear(s.active)
	s.mu.Unlock()
	if err := s.cleanup(now); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		if tc.active {
			if _, err := os.Stat(s.jobDir(tc.m.ID)); !os.IsNotExist(err) {
				t.Errorf("released %q retained: %v", tc.m.ID, err)
			}
		}
	}
}

func TestCleanupMalformedMetadata(t *testing.T) {
	s := testServer(t)
	now := time.Now()
	for _, tc := range []struct {
		id           string
		corrupt, old bool
	}{
		{id: "recent missing"}, {id: "recent corrupt", corrupt: true},
		{id: "old missing", old: true}, {id: "old corrupt", corrupt: true, old: true},
	} {
		dir := s.jobDir(tc.id)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if tc.corrupt {
			if err := os.WriteFile(filepath.Join(dir, "metadata.json"), []byte("not JSON"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		stamp := now
		if tc.old {
			stamp = now.Add(-7 * 24 * time.Hour)
		}
		if err := os.Chtimes(dir, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	nonJob := filepath.Join(s.dataDir, "jobs", "not-a-directory")
	if err := os.WriteFile(nonJob, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.cleanup(now); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"recent missing", "recent corrupt"} {
		if _, err := os.Stat(s.jobDir(id)); err != nil {
			t.Errorf("recent malformed %q removed: %v", id, err)
		}
	}
	for _, id := range []string{"old missing", "old corrupt"} {
		if _, err := os.Stat(s.jobDir(id)); !os.IsNotExist(err) {
			t.Errorf("old malformed %q retained: %v", id, err)
		}
	}
	if _, err := os.Stat(nonJob); err != nil {
		t.Errorf("non-job file removed: %v", err)
	}
}

func TestRestartPersistenceAndStartupCleanup(t *testing.T) {
	s := testServer(t)
	if w := requestRender(t, s, "persisted/slash"); w.Code != http.StatusOK {
		t.Fatalf("render = %d", w.Code)
	}
	if w := outputRequest(s, http.MethodGet, "persisted/slash", nil); w.Code != http.StatusOK {
		t.Fatalf("download = %d", w.Code)
	}
	original := metadataFor(t, s, "persisted/slash")
	now := time.Now().UTC()
	expiredDownload := now.Add(-60 * time.Minute)
	recentDownload := now.Add(-58 * time.Minute)
	seedJob(t, s, jobMetadata{ID: "old", CreatedAt: now.Add(-8 * 24 * time.Hour), Ready: true})
	seedJob(t, s, jobMetadata{ID: "download expired", CreatedAt: now.Add(-time.Hour), FreeableAt: &expiredDownload, Ready: true})
	seedJob(t, s, jobMetadata{ID: "download recent", CreatedAt: now.Add(-time.Hour), FreeableAt: &recentDownload, Ready: true})
	restarted, err := NewServer(Config{DataDir: s.dataDir})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"old", "download expired"} {
		if _, err := os.Stat(restarted.jobDir(id)); !os.IsNotExist(err) {
			t.Errorf("startup retained expired %q: %v", id, err)
		}
		if outputRequest(restarted, http.MethodGet, id, nil).Code != http.StatusNotFound {
			t.Errorf("expired %q downloadable", id)
		}
	}
	restored := metadataFor(t, restarted, "persisted/slash")
	if restored.ID != original.ID || !restored.Ready || !restored.CreatedAt.Equal(original.CreatedAt) || restored.FreeableAt == nil || original.FreeableAt == nil || !restored.FreeableAt.Equal(*original.FreeableAt) {
		t.Fatalf("metadata changed after restart: before=%+v after=%+v", original, restored)
	}
	if outputRequest(restarted, http.MethodGet, "persisted/slash", nil).Body.String() != testOutput {
		t.Fatal("persisted output not served")
	}
	if m := metadataFor(t, restarted, "persisted/slash"); !m.FreeableAt.Equal(*original.FreeableAt) {
		t.Fatal("restart download extended retention")
	}
	if err := restarted.cleanup(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(restarted.jobDir("download recent")); !os.IsNotExist(err) {
		t.Fatalf("persisted downloaded TTL not enforced: %v", err)
	}
}

func TestExpiredIdentityCanBeReused(t *testing.T) {
	s := testServer(t)
	seedJob(t, s, jobMetadata{ID: "reuse", CreatedAt: time.Now().Add(-8 * 24 * time.Hour), Ready: true})
	w := requestRender(t, s, "reuse")
	if w.Code != http.StatusOK {
		t.Fatalf("expired identity = %d: %s", w.Code, w.Body.String())
	}
	m := metadataFor(t, s, "reuse")
	if !m.Ready || expired(m, time.Now()) || m.FreeableAt != nil {
		t.Fatalf("reused metadata = %+v", m)
	}
}

func TestOutputRejectsInvalidStorage(t *testing.T) {
	for _, kind := range []string{"missing", "not ready", "expired", "wrong identity", "missing output", "directory output", "corrupt metadata"} {
		t.Run(kind, func(t *testing.T) {
			s := testServer(t)
			m := jobMetadata{ID: "movie", CreatedAt: time.Now(), Ready: true}
			if kind != "missing" {
				seedJob(t, s, m)
			}
			switch kind {
			case "not ready":
				m.Ready = false
			case "expired":
				m.CreatedAt = time.Now().Add(-8 * 24 * time.Hour)
			case "wrong identity":
				m.ID = "other"
			case "missing output", "directory output":
				path := filepath.Join(s.jobDir("movie"), "output.mp4")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if kind == "directory output" {
					if err := os.Mkdir(path, 0o755); err != nil {
						t.Fatal(err)
					}
				}
			case "corrupt metadata":
				if err := os.WriteFile(filepath.Join(s.jobDir("movie"), "metadata.json"), []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "not ready" || kind == "expired" || kind == "wrong identity" {
				if err := writeMetadata(s.jobDir("movie"), m); err != nil {
					t.Fatal(err)
				}
			}
			if got := outputRequest(s, http.MethodGet, "movie", nil).Code; got != http.StatusNotFound {
				t.Errorf("status = %d, want 404", got)
			}
			if len(s.active) != 0 {
				t.Errorf("invalid output leaked reservation: %v", s.active)
			}
		})
	}
}

func TestNewServerStorageAndCleanupCancellation(t *testing.T) {
	s := testServer(t)
	for _, child := range []string{"jobs", "media"} {
		info, err := os.Stat(filepath.Join(s.dataDir, child))
		if err != nil || !info.IsDir() {
			t.Fatalf("storage %q not initialized: %v", child, err)
		}
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewServer(Config{DataDir: blocked}); err == nil {
		t.Fatal("file data directory accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); s.RunCleanup(ctx) }()
	cancel()
	awaitSignal(t, done)
}
