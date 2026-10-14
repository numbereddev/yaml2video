# YAML to Video

Create MP4 videos from YAML. A **video file** describes the content; a **template file** controls the layout and styling. Run it as a command-line tool or an internal rendering API.

## Quick start with Docker

Clone this repository and open a terminal in its folder. You need Docker with Compose and `make`; Go, FFmpeg, and fonts are included in the image.

```sh
make build
make run ARGS="examples/simple/video-simple.yaml"
```

`make run` runs yaml2video in the container with your current folder.  
The example's video YAML includes `template_file: template-simple.yaml`, resolved beside the video YAML, so `-t` is optional. The result is `examples/simple/output.mp4` on your computer.

For your own files, pass CLI arguments through `ARGS`:

```sh
make run ARGS="-t template.yaml -o output.mp4 video.yaml"
```

Omit `-t` if your video YAML includes `template_file`; otherwise, `-t` is required. An explicit `-t` overrides `template_file`, but the required `video.template` identifier must still match the selected template's `template_id`.

Add `-n` to check the YAML and preview the rendering commands without creating a video. See the [user guide](docs/user-guide.md) for writing YAML and using media files.

## Container

Available Make commands (`make` defaults to `make help`). Container commands use Docker Compose and the same image:

| Command | What it does |
| --- | --- |
| `make help` | Show available Make commands |
| `make build` | Build or rebuild the image |
| `make run ARGS="..."` | Run the CLI with the current folder mounted at `/data` (defaults to `--help`) |
| `make test` | Run Go tests against the current checkout |
| `make up` | Start the API server at `http://127.0.0.1:8080` |
| `make down` | Stop the server and keep its data |
| `make down-volumes` | Stop the server and **delete its stored data** |
| `make logs` | Follow container logs; press Ctrl-C to stop following |
| `make ps` | Show service status |
| `make config` | Show the resolved Docker Compose configuration |
| `make bash` | Open an optional container shell with the current folder mounted at `/data` |

Build the image before starting the server. Server data is stored in a named Docker volume, separate from your input files.

### Server input files

`make up` mounts `./examples/` from your computer read-only at `/data` in the container. Put your video YAML, template YAML, and referenced media in that folder, ideally together in a project subfolder. For example, `examples/simple/video-simple.yaml` is available to the server as `/data/simple/video-simple.yaml` (or `simple/video-simple.yaml`, relative to the server's `/data` working directory).

Override the host folder with `MOUNT_DIR` (not `PATH`, which controls executable lookup):

```sh
make up MOUNT_DIR=/absolute/path/to/my-video-files
make config MOUNT_DIR=/absolute/path/to/my-video-files
```

Relative `MOUNT_DIR` values resolve from the directory containing `docker-compose.yaml`. The container path remains `/data` regardless of the host folder. The API reads input files from this mount.

Use the same `MOUNT_DIR` value for subsequent Compose commands. With Docker Compose directly, use `MOUNT_DIR=/absolute/path/to/my-video-files docker compose up -d`.

`make run` and `make bash` still mount the repository's current folder at `/data` instead, preserving the CLI examples above.

**The API has no authentication and is for trusted, internal use only.** Keep it private; do not expose it to the internet. See the [API documentation](docs/server-api.md) for requests, media provisioning, and downloads.

### Using without cloning

To render without cloning the repository, run the published image from the folder containing your YAML and media files:

```sh
docker run --rm -v "$PWD:/data" ghcr.io/ondics/yaml2video:latest -t template.yaml video.yaml
```

## Run without Docker

Download the executable for your platform from [releases](https://github.com/ondics/yaml2video/releases/latest) and install [FFmpeg](https://ffmpeg.org/download.html) with the `ass` filter enabled. Make sure `ffmpeg` is on your `PATH`.

```sh
# macOS / Linux
chmod +x ./yaml2video
./yaml2video -t template.yaml video.yaml
```

On Windows, use `yaml2video.exe` instead. Without `-o`, the output is `output.mp4` next to the video YAML file.

## Documentation

- [User guide](docs/user-guide.md) and [examples](examples/)
- [Server API](docs/server-api.md)
- YAML schemas: [video](docs/video.schema.json) · [template](docs/template.schema.json)
- [YAML format and rendering reference](docs/reference.md)
- [Contributing and development setup](CONTRIBUTING.md)

## License

This project is licensed under [MIT](LICENSE).
