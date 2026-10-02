# Usage and project structure

A project is a YAML document that describes one output video. A conventional project uses `scenes`; a presentation-style project uses `slides` plus one or more template files. Both forms become the same render plan.

## Render a project

```sh
yaml2video project.yaml
yaml2video project.yaml -o launch.mp4 -work-dir build/yaml2video
yaml2video -n project.yaml
```

`-n` is the safest first command: it parses and validates the project, prints a timeline summary, and prints the FFmpeg commands without writing intermediate files or invoking FFmpeg.

Paths in YAML are passed to FFmpeg as written. Run the command from a directory where the paths resolve, or use paths relative to your working directory.

## Minimal conventional project

```yaml
version: 1
video:
  width: 1920
  height: 1080
  fps: 30
  background: "#101010"

scenes:
  - id: opening
    duration: 3
    layers:
      - type: image
        path: assets/cover.png
        fit: cover
        transform: center
```

`video.width`, `video.height`, and `video.fps` must be positive. Every project needs at least one scene; each scene needs a unique `id` and a positive duration in seconds. A scene uses its own `background` when supplied, otherwise it uses `video.background`.

## Defaults

`defaults.text` is used by every text layer. `defaults.subtitle` uses the same font, size, color, and background fields. The renderer currently places subtitles at the bottom center of the video; use the subtitle defaults to set their typography. Its `transform` and `max_width` fields are accepted but do not currently change subtitle placement or wrapping.

```yaml
defaults:
  text:
    font: "DejaVu Sans"
    size: 42
    color: white
    background:
      color: black
      opacity: 0.6
      padding: [12, 6]
      radius: 8
  subtitle:
    font: "DejaVu Sans"
    size: 28
    color: white
    transform: bottom-center
    max_width: 1200
```

## Audio

A top-level `music` track loops for the complete finished video. Scene `audio` layers start at the beginning of their scene; multiple tracks are mixed. Scene audio and video layers can use `source_offset` and `duration` to select part of a source.

```yaml
music:
  path: assets/music.mp3
  volume: 0.35
  fade_out: 1

scenes:
  - id: narration
    duration: 5
    layers:
      - type: audio
        path: assets/narration.wav
        source_offset: 0.25
        duration: 4.5
        volume: 0.9
```

`fade_out` is measured in seconds. Audio and video layer durations must be positive and no longer than their containing scene. Their source offsets must not be negative.

## Transitions and timeline behavior

A scene can define only an outgoing transition, which overlaps it with the next scene and shortens the total video by the transition duration. Supported scene transition types are `fade` and `dissolve`.

```yaml
- id: first
  duration: 3
  transition:
    out:
      type: dissolve
      duration: 0.5
```

Subtitle transitions are independent of scene transitions and currently support `fade` only. If a subtitle has no explicit `offset`, it begins immediately after the previous subtitle ends. An explicit `offset` resets that placement within the scene.

## Examples

From the repository root:

```sh
go run . -n examples/project/example.yaml
go run . -n -t examples/templates/appdemo-template.yaml examples/templates/appdemo-project.yaml
```

For the complete set of YAML fields and validation rules, see the [YAML feature reference](yaml-reference.md). For `slides`, see [slide templates](templates.md).
