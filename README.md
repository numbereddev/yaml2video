# YAML to Video

Create MP4 videos from YAML. A **video file** describes the content; a **template file** controls the layout and styling. Run it as a command-line tool or an internal rendering API.

## Quick start with Docker

Clone this repository and open a terminal in its folder. You need Docker with Compose and `make`; Go, FFmpeg, and fonts are included in the image.

```sh
make build
make bash
```

The shell opens inside the container at `/data`, with your current project folder mounted there. Run the included example:

```sh
yaml2video -t examples/simple/template-simple.yaml examples/simple/video-simple.yaml
```

The result is `examples/simple/output.mp4` on your computer. Type `exit` to leave the container.

For your own files, run this inside the same shell:

```sh
yaml2video -t template.yaml -o output.mp4 video.yaml
```

Add `-n` to check the YAML and preview the rendering commands without creating a video. See the [user guide](docs/user-guide.md) for writing YAML and using media files.

## Container

All Make commands run in Docker and use the same image:

| Command | What it does |
| --- | --- |
| `make build` | Build or rebuild the image |
| `make bash` | Open a shell with the current folder mounted at `/data` |
| `make tests` | Run Go tests against the current checkout |
| `make up` | Start the API server at `http://127.0.0.1:8080` |
| `make down` | Stop the server and keep its data |
| `make down-volumes` | Stop the server and **delete its stored data** |

Build the image before starting the server. Server data is stored in a named Docker volume, separate from the folder mounted by `make bash`.

**The API has no authentication and is for trusted, internal use only.** Keep it private; do not expose it to the internet. See the [API documentation](docs/server-api.md) for requests, media provisioning, and downloads.

To render without cloning the repository, run the published image from the folder containing your YAML and media files:

```sh
docker run --rm -v "$PWD:/data" ghcr.io/ondics/yaml2video:main -t template.yaml video.yaml
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
