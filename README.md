# Yaml to Video

`yaml2video` renders declarative YAML projects into MP4 videos using FFmpeg. Define a timeline of scenes, media, text, subtitles, shapes, transitions, and audio—or use reusable slide templates for presentation-style videos.

## Requirements

- [Go](https://go.dev/) 1.27.1 or later to build from source.
- [FFmpeg](https://ffmpeg.org/) on `PATH` to render videos.
- An FFmpeg build with the `ass` filter (libass enabled) when a project contains text or subtitles.

On macOS, Homebrew provides FFmpeg with `brew install ffmpeg`. Verify the text-rendering requirement with:

```sh
ffmpeg -hide_banner -filters | grep ass
```

## Install

Download a binary for your platform from [Releases](https://github.com/ondics/yaml2video/releases), or install the latest module with Go:

```sh
go install github.com/ondics/yaml2video@latest
```

For local development:

```sh
git clone https://github.com/ondics/yaml2video.git
cd yaml2video
go build .
```

## Quick start

Create `video.yaml`:

```yaml
version: 1
video:
  width: 1280
  height: 720
  fps: 30
  background: "#101010"

defaults:
  text:
    font: Arial
    size: 48
    color: white

scenes:
  - id: title
    duration: 3
    layers:
      - type: image
        path: cover.jpg
        fit: cover
        transform: center
      - type: text
        text: ["Hello, yaml2video"]
        transform: bottom-center
```

Inspect the generated render plan without creating files:

```sh
yaml2video -n video.yaml
```

Render the video (by default, `output.mp4`):

```sh
yaml2video -o welcome.mp4 video.yaml
```

Intermediate ASS and scene files are stored in `.out/` by default. Change this with `-work-dir`.

## CLI

```text
yaml2video [-n] [-t template.yaml]... [-work-dir .out] [-o output.mp4] <project.yaml>
```

The project path can be first or last. Options:

| Option | Description |
| --- | --- |
| `-n` | Validate, compile, and print the render summary and exact FFmpeg commands; do not render. |
| `-o <file>` | Final output path. Defaults to `output.mp4`. |
| `-work-dir <dir>` | Directory for intermediate ASS and scene files. Defaults to `.out`. |
| `-t <file>` | Slide-template YAML file. Repeat to compose multiple template catalogs. |

## Documentation

- [Usage and project structure](docs/usage.md)
- [YAML feature reference](docs/yaml-reference.md)
- [Slide templates](docs/templates.md)
- [Runnable examples](examples/README.md)

## Development

```sh
make test
make build
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow and pull-request expectations.

## License

This project is licensed under the [MIT License](LICENSE).
