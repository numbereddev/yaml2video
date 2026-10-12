package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderWithRealFFmpegAndDownload(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	s, err := NewServer(Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	// The public schema requires a slide image; generate it locally rather than
	// depending on example assets or network access.
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.Set(x, y, color.RGBA{B: 255, A: 255})
		}
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.dataDir, "media", "slide.png"), imageData.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	// A small subset of examples/simple, retaining intro, slide, and outro text.
	video := []byte(`
$schema: video.schema.json
version: 2
template: integration
metadata:
  title: Integration test
  language: en
content:
  intro:
    title: Hello
  slides:
    - image: slide.png
      text: Slide
  outro:
    cta: Done
`)
	template := []byte(`
$schema: template.schema.json
version: 2
template_id: integration
description: Small local FFmpeg integration fixture
format:
  aspect_ratio: "1:1"
  width: 128
  height: 128
  fps: 10
defaults:
  theme:
    background: "#000000"
    text_color: "#FFFFFF"
    accent_color: "#FFFFFF"
  typography:
    font_family: Arial
    title_size: 16
    body_size: 16
  timing:
    intro_duration: 0.2s
    slide_duration: 0.2s
    outro_duration: 0.2s
    transition:
      type: cut
      duration: 0s
composition:
  sections:
    - id: intro
      role: intro
      source: content.intro
    - id: slides
      role: slide
      source: content.slides
      repeat: true
    - id: outro
      role: outro
      source: content.outro
layout:
  intro:
    background: "#000000"
    title:
      placement:
        anchor: center
  slide:
    background: "#000000"
    image:
      placement:
        anchor: center
      max_width_ratio: 0.5
      max_height_ratio: 0.5
    text:
      placement:
        anchor: bottom-center
        offset_y: -16
  outro:
    background: "#000000"
    cta:
      placement:
        anchor: center
`)
	id := "real ffmpeg #1"
	// JSON marshaling []byte supplies the API's base64-encoded YAML fields.
	body, err := json.Marshal(renderRequest{ID: id, Video: video, Template: template})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	handler := s.Handler()
	request := httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(body)).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	stream := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	handler.ServeHTTP(stream, request)
	if stream.Code != http.StatusOK || stream.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("render response = %d %v: %s", stream.Code, stream.Header(), stream.Body.String())
	}
	events := parseSSE(t, stream.Body.String())
	stages := make(map[string]bool)
	for _, event := range events {
		if event.name == "error" {
			t.Fatalf("render error: %v", event.data)
		}
		if event.data["id"] != id {
			t.Fatalf("event identity = %v, want %q", event.data["id"], id)
		}
		if event.name == "progress" {
			stage, _ := event.data["stage"].(string)
			stages[stage] = true
		}
	}
	for _, stage := range []string{"starting", "scene", "final", "done"} {
		if !stages[stage] {
			t.Errorf("missing %q progress: %s", stage, stream.Body.String())
		}
	}
	complete := events[len(events)-1]
	wantURL := "/output/" + url.PathEscape(id)
	if complete.name != "complete" || complete.data["url"] != wantURL {
		t.Fatalf("completion = %#v, want URL %q", complete, wantURL)
	}
	if len(stream.frames) != len(events) {
		t.Fatalf("flushes = %d, events = %d", len(stream.frames), len(events))
	}
	metadata := metadataFor(t, s, id)
	if !metadata.Ready || metadata.FreeableAt != nil {
		t.Fatalf("metadata before download = %+v", metadata)
	}
	for i, text := range []string{"Hello", "Slide", "Done"} {
		assPath := filepath.Join(s.jobDir(id), "work", "ass", fmt.Sprintf("scene-%04d.ass", i))
		ass, err := os.ReadFile(assPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(ass), text) {
			t.Errorf("%s does not contain %q", assPath, text)
		}
		path := filepath.Join(s.jobDir(id), "work", "scenes", fmt.Sprintf("scene-%04d.mkv", i))
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("missing or empty intermediate %s: %v", path, err)
		}
	}

	before := time.Now()
	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, complete.data["url"].(string), nil).WithContext(ctx))
	if download.Code != http.StatusOK || download.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("download response = %d %v: %s", download.Code, download.Header(), download.Body.String())
	}
	artifact, err := os.ReadFile(filepath.Join(s.jobDir(id), "output.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(download.Body.Bytes(), artifact) {
		t.Fatal("download differs from rendered artifact")
	}
	if len(artifact) < 12 || string(artifact[4:8]) != "ftyp" {
		t.Fatal("download is not an MP4 container")
	}
	marked := metadataFor(t, s, id)
	if marked.FreeableAt == nil || marked.FreeableAt.Before(before) || marked.FreeableAt.After(time.Now()) {
		t.Fatalf("freeable timestamp = %v", marked.FreeableAt)
	}

	// Decode the entire downloaded video, not just its container header. Six
	// 128x128 RGB frames prove that all three short scenes reached the final MP4.
	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-i", "pipe:0", "-map", "0:v:0", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
	cmd.Stdin = bytes.NewReader(download.Body.Bytes())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	frames, err := cmd.Output()
	if err != nil {
		t.Fatalf("decode downloaded MP4: %v: %s", err, stderr.String())
	}
	if want := 6 * 128 * 128 * 3; len(frames) != want {
		t.Fatalf("decoded video bytes = %d, want %d", len(frames), want)
	}
}
