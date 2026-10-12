# YAML to Video

Turn a YAML video and a template into an MP4. No coding is needed: download the app, install FFmpeg, and start with the included example.

## 1. Download yaml2video

Go to [releases](https://github.com/ondics/yaml2video/releases/latest), download the file for your operating system and CPU.

Keep the `yaml2video` program somewhere convenient. On macOS or Linux, if needed, make it executable:

```sh
chmod +x ./yaml2video
```

On Windows, use `yaml2video.exe` in the commands below.

## 2. Install FFmpeg (full build)

`yaml2video` uses FFmpeg to create the video. Install a full FFmpeg build and make sure the `ffmpeg` command is available in your terminal:

- **Windows:** install the [full build from Gyan](https://www.gyan.dev/ffmpeg/builds/) ("release full"), then reopen your terminal.
- **macOS:** install [Homebrew](https://brew.sh/) if needed, then run `brew install ffmpeg-full`
- **Linux:** install the full FFmpeg package provided for your distribution. For Debian/Ubuntu, run `sudo apt install ffmpeg`.

To check that FFmpeg is ready, run `ffmpeg -version`. If the command is not found, follow the installation instructions for your system to add FFmpeg to `PATH`.

## 3. Make a video

Open a terminal in the folder where you extracted the example files, or run these commands from the project folder. The example video and template are in `examples/simple/`:

> [!NOTE]
> On Windows, use `yaml2video.exe` in the commands below.

```sh
./yaml2video -n -t examples/simple/template-simple.yaml examples/simple/video-simple.yaml
./yaml2video -t examples/simple/template-simple.yaml examples/simple/video-simple.yaml
```

The first command checks the files and shows what will be rendered without creating a video. The second renders the video. By default, the output is `output.mp4` next to the video YAML file.

## Use your own files

Create a video YAML and a template YAML, then run:

```sh
./yaml2video -t template.yaml video.yaml
```

Keep media files (images and audio) where the paths in `video.yaml` expect them. The [step-by-step guide](docs/user-guide.md) explains how to create your own files, and the [examples](examples/) provide more to try.

## More information

- [User guide](docs/user-guide.md)
- [Container usage](#container)
- [Internal render server](#internal-render-server) and [API contract](docs/server-api.md)
- [Build from source](#build-from-source)

<details>
<summary>Container usage</summary>

Prefer not to install FFmpeg? The GHCR image includes the app and its rendering dependencies. From the folder containing your YAML files and media, run:

```sh
docker pull ghcr.io/ondics/yaml2video:main
docker run --rm -v "$PWD:/data" ghcr.io/ondics/yaml2video:main -t /data/template.yaml /data/video.yaml
```

Replace `template.yaml` and `video.yaml` with your filenames. The output is written into the mounted folder so it stays on your computer. Published releases are also available using their release tag instead of `main`.

To build and render the included example from the project folder, only Docker with Compose is required:

```sh
docker compose up --build --abort-on-container-exit --exit-code-from yaml2video
```

This is a one-shot CLI job, not a background server. It exits after rendering and writes `examples/simple/output.mp4` into the project folder. Go, FFmpeg, and fonts are included in the containers.

For your own files or a validation-only run:

```sh
docker compose run --rm --build yaml2video -t template.yaml -o output.mp4 video.yaml
docker compose run --rm --build yaml2video -n -t examples/simple/template-simple.yaml examples/simple/video-simple.yaml
```

All Make targets also run in containers:

```sh
make run              # render the included example using go run
make build            # build ./yaml2video for the container's Linux architecture
make install          # install the Linux binary into .out/bin/yaml2video
make test             # run Go tests with FFmpeg available
make docker-build     # build the runtime image
make docker-run       # render the included example using the runtime image
```

Set `ARGS` for `make run`, or `DOCKER_ARGS` for `make docker-run`, to pass different paths or options:

```sh
make run ARGS='-n -t examples/simple/template-simple.yaml examples/simple/video-simple.yaml'
make docker-run DOCKER_ARGS='-t template.yaml -o output.mp4 video.yaml'
```

The runtime service mounts the project folder at `/data`; the development service mounts it at `/src` and keeps Go build and module caches in named Docker volumes. Paths outside the project must be mounted explicitly. `make build` and `make install` produce Linux binaries, not native macOS or Windows executables. Override `DOCKER_IMAGE` to change the image tag or `COMPOSE` to change the Compose command.

Run `docker compose down` to remove the job container and network. Add `--volumes` to also clear the Go caches.

</details>

<details>
<summary>Build from source</summary>

Install Go 1.27.1 or later, then run:

```sh
go build -o yaml2video ./src/cmd/yaml2video
go test ./...
```

</details>

<details>
<summary>Document schemas and renderer details</summary>

The [video schema](docs/video.schema.json) and [template schema](docs/template.schema.json) describe the YAML formats. See the [format specification](docs/video2yaml-spec.md) and [renderer specification](docs/renderer-spec.md) for implementation details.

</details>

## Internal render server

The server API is **trusted, unauthenticated, and internal-only**. Do not expose it to the internet or other untrusted clients. The serve Compose configuration publishes port 8080 on loopback only (`127.0.0.1:8080:8080`); keep that binding and any proxy private.


From the project folder:

```sh
docker compose -f docker-compose.serve.yaml up --build -d
docker compose -f docker-compose.serve.yaml down
```

The serve configuration sets `YAML2VIDEO_DATA_DIR=/var/lib/yaml2video` and mounts the persistent named volume `yaml2video-data` there (Compose prefixes the volume name with its project name). Normal `down` preserves it; `down --volumes` deletes its stored data. This is separate from the one-shot CLI containers above.

Server storage uses `YAML2VIDEO_DATA_DIR` when set; otherwise it uses `/var/lib/yaml2video` on Linux, `/Library/Application Support/yaml2video` on macOS, or `%ProgramData%\yaml2video` on Windows (falling back to `C:\ProgramData\yaml2video` when `ProgramData` is unset). Ensure the server can write to this directory. Relative media paths resolve under `<dataDir>/media`: callers must provision media there or mount it there themselves. The API provides no media upload or remote fetch.

The API accepts `POST /render` JSON with a `name` or `id` and base64-encoded YAML in `video` and `template` (compatible with the existing Go `[]byte` JSON fields). It streams SSE `progress`, `complete`, and `error` events plus heartbeat comments; disconnecting cancels rendering. `GET /output/{id}` downloads an MP4 attachment. An occupied ID returns HTTP 409; invalid requests return HTTP 400.

Jobs expire seven days after creation. A successful full, non-range GET marks a job freeable for deletion within an hour; HEAD, partial, and failed downloads do not. Cleanup runs every minute and lifecycle metadata persists across restarts. See the [full API contract and curl example](docs/server-api.md) for payloads, storage provisioning, and download semantics.

## License

[MIT](LICENSE)
