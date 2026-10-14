# Contributing to yaml2video

Keep changes focused, follow the existing code style, and add tests for behavior changes.

## Develop with Docker

Install Docker with Compose and `make`, then clone the repository and enter its folder. No local Go or FFmpeg installation is required.

```sh
make build           # build the shared application and development image
make tests           # runs `go test ./...`, if you updated Dockerfile, make sure to run `make build`
make bash            # open a shell with the checkout mounted at /data
```

`make tests` mounts the checkout at `/src`. In the `make bash` shell, work at `/data` so edits and generated files are saved to your checkout. The image also contains a copy of the source at `/src`; do not edit that copy.

Inside the shell, run development commands against your current source:

```sh
gofmt -w src
go vet ./...
go test -race ./...
go build ./...
go run ./src/cmd/yaml2video -n -t examples/simple/template-simple.yaml examples/simple/video-simple.yaml
```

Use `go run` to try source changes. The installed `yaml2video` executable is built into the image and only changes when you run `make build` again. Type `exit` to leave the shell.

For the API server, run `make up` after building; `make down` stops it without deleting data. See [container usage](README.md#container) for details. The API is unauthenticated and must remain private.

## Develop without Docker

Install Go **1.27.1 or later** (see `go.mod`) and clone the repository. From the project folder:

```sh
go mod download
go test ./...
go run ./src/cmd/yaml2video -n -t examples/simple/template-simple.yaml examples/simple/video-simple.yaml
```

For actual rendering, install FFmpeg and fonts. Text and subtitles require FFmpeg's `ass` filter (libass enabled). Dry-run mode (`-n`) validates YAML and prints rendering commands without creating a video.

## Before opening a pull request

Run these commands from the project folder, either locally or inside `make bash`:

```sh
gofmt -w src
go vet ./...
go test -race ./...
go build ./...
go run ./src/cmd/yaml2video -n -t examples/normal/template-normal.yaml examples/normal/video-normal.yaml
go run ./src/cmd/yaml2video -n -t examples/complex/template-complex.yaml examples/complex/video-complex.yaml
```

CI checks formatting, runs vet and race-enabled tests, builds all packages, and validates the normal and complex examples. `make tests` is a quick test run; it does not run all CI checks.

- Add or update tests for behavior changes, especially validation, rendering, and the API.
- Update `README.md` or `docs/` when changing CLI options, YAML formats, or API behavior.
- Do not commit generated videos, build output, or temporary files.
- Avoid unrelated refactors in the same pull request.
- Use the [pull-request template](.github/pull_request_template.md) to explain the change and report what you tested.

## Reporting security issues

Do not post potential security vulnerabilities in public issues. Contact the repository maintainers privately through the contact method listed on the project's GitHub profile.
