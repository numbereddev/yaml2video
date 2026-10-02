package render

import (
	"strings"
	"testing"

	"github.com/ondics/yaml2video/project"
)

func TestCompileMediaAndCircleLayers(t *testing.T) {
	value, err := project.Parse([]byte(`
music:
  path: bed.mp3
  volume: 0.25
video:
  width: 320
  height: 240
  fps: 30
  background: "#000000"
scenes:
  - id: first
    duration: 2
    layers:
      - type: video
        path: clip.mp4
        source_offset: 0.5
        duration: 1.25
        fit: contain
        width: 320
        height: 240
        transform:
          x: 0.5
          y: 0.5
          scale_x: 0.5
          scale_y: 0.5
      - type: circle
        color: "#ff0000"
        opacity: 0.5
        width: 320
        height: 240
        transform:
          x: 0.25
          y: 0.5
          scale_x: 0.25
          scale_y: 0.25
    transition:
      out:
        type: fade
        duration: 0.5
  - id: second
    duration: 2
    layers:
      - type: audio
        path: narration.wav
        source_offset: 0.25
        duration: 0.7
        volume: 0.7
`))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Compile(value, Options{
		WorkDir: ".out/test",
		Output:  "test.mp4",
	})
	if err != nil {
		t.Fatal(err)
	}

	audioLayers := plan.Scenes[1].Audio
	if len(audioLayers) != 1 {
		t.Fatalf("expected one audio layer, got %#v", audioLayers)
	}

	audioLayer := audioLayers[0]
	if audioLayer.Volume != 0.7 {
		t.Errorf("audio volume = %v, want 0.7", audioLayer.Volume)
	}
	if audioLayer.SourceOffset.Seconds() != 0.25 {
		t.Errorf("audio source offset = %s, want 250ms", audioLayer.SourceOffset)
	}
	if audioLayer.Duration.Seconds() != 0.7 {
		t.Errorf("audio duration = %s, want 700ms", audioLayer.Duration)
	}

	videoLayer := plan.Scenes[0].Layers[0]
	if videoLayer.SourceOffset.Seconds() != 0.5 {
		t.Errorf("video source offset = %s, want 500ms", videoLayer.SourceOffset)
	}
	if videoLayer.Duration.Seconds() != 1.25 {
		t.Errorf("video duration = %s, want 1.25s", videoLayer.Duration)
	}
	if plan.Scenes[1].Start.Seconds() != 1.5 {
		t.Fatalf("expected second scene audio to start at 1.5s after the transition, got %s", plan.Scenes[1].Start)
	}

	commands, err := plan.Commands()
	if err != nil {
		t.Fatal(err)
	}
	joinedCommands := make([]string, len(commands))
	for index, command := range commands {
		joinedCommands[index] = strings.Join(command, " ")
	}

	output := strings.Join(joinedCommands, "\n")
	expectedFilters := []string{
		"geq",
		"trim",
		"duration=1.250000",
		"duration=0.700000",
		"-ss 0.500000",
		"-ss 0.250000",
		"adelay",
		"delays=1500",
		"amix",
	}
	for _, filter := range expectedFilters {
		if !strings.Contains(output, filter) {
			t.Errorf("generated commands do not contain %q:\n%s", filter, output)
		}
	}
}
