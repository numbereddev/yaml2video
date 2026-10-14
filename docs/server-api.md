# Render API

For trusted internal clients only. No authentication.

## Local input files in Docker

The Compose server mounts the host's `./examples/` folder at `/data`, which is also its working directory. Place video YAML files, templates, and their media there. For example, `examples/simple/video-simple.yaml` on the host becomes `/data/simple/video-simple.yaml` in the container. Host paths outside this mount are not available through it.

To mount another folder, start the server with `make up MOUNT_DIR=/absolute/path/to/files`. The container path is still `/data`. Relative mount paths resolve beside `docker-compose.yaml`. The mount is writable, so local render intermediates or output placed there will appear on the host.

This input mount is separate from the server's named data volume at `/var/lib/yaml2video`. `POST /render` continues to resolve relative media paths under `/var/lib/yaml2video/media`, not `/data`.

## GET /render/local

Render YAML files already present on the server (not uploaded by the request):

```sh
curl --fail -N --get http://localhost:8080/render/local \
  --data-urlencode 'path=simple/video-simple.yaml'
```

| Query parameter | Meaning |
| --- | --- |
| `path` | Required video YAML path. Relative paths resolve from the server's working directory (`/data` in Compose). |
| `template` | Optional template YAML path, resolved from the same working directory. Overrides the video's `template_file`. |
| `dir` | Optional directory for intermediate render files and base for relative `template_file` references. Relative paths resolve from the server's working directory. |

Without `template`, the video YAML must contain `template_file`. Without `dir`, that reference resolves beside the video YAML and intermediates go into `.out` beside it. Each request uses an isolated subfolder within the intermediate directory. Relative media paths always resolve beside the video YAML, even when `dir` is supplied.

For example, an explicit template and work directory:

```sh
curl --fail -N --get http://localhost:8080/render/local \
  --data-urlencode 'path=simple/video-simple.yaml' \
  --data-urlencode 'template=simple/template-simple.yaml' \
  --data-urlencode 'dir=work'
```

The server generates an ID for each request. Responses use the same `progress`, `complete`, and `error` SSE events as `POST /render`; the `complete` event includes the download URL. Completed MP4s are stored in the server's data volume, not alongside the source YAML, and use the same retention rules. Disconnecting cancels rendering. Missing or invalid files return `400` JSON before streaming starts.

**Trusted clients only:** this endpoint can read paths accessible to the server process; `/data` is a mount location, not a filesystem access sandbox. It writes render intermediates and starts a render even though its HTTP method is GET.

## POST /render

Send `Content-Type: application/json`. `video` and `template` must be JSON objects matching [video.schema.json](video.schema.json) and [template.schema.json](template.schema.json). `$schema` is an optional editor hint and is ignored by the server; validation uses embedded schemas. No base64 or YAML strings.

Request structure (replace the empty objects with complete documents):

```json
{
  "id": "demo-001",
  "video": {},
  "template": {}
}
```

All fields are required. `id` must be valid UTF-8, at most 512 bytes, and contain no NUL or line breaks. The request body limit is 16 MiB. Referenced media must already exist on the server; relative paths resolve under its media directory.

**Response:** `200 OK`, `Content-Type: text/event-stream`. Keep the connection open until `complete` or `error`; disconnecting cancels rendering.

| SSE event | JSON data |
| --- | --- |
| `progress` | `id`, `stage`, `percent` (0–100), plus optional render details |
| `complete` | `id`, `url` pointing to `/renders/{id}` |
| `error` | `id`, `error` describing the failure |

```text
event: progress
data: {"id":"demo-001","stage":"starting","percent":0}

event: complete
data: {"id":"demo-001","url":"/renders/demo-001"}

```

Ignore SSE heartbeat comments (`: heartbeat`). Failures after streaming starts use the `error` event, not a new HTTP status.

Before streaming, errors return JSON such as `{"error":"message"}`:

| Status | Meaning |
| --- | --- |
| `400` | Invalid request or video/template document |
| `409` | ID already occupied |
| `413` | Request body too large |
| `500` | Server failure |

## GET /renders

List completed, unexpired renders, newest first. Returns `200 OK` with a JSON array (`[]` if none are available):

```json
[
  {
    "id": "demo-001",
    "created_at": "2026-10-14T12:00:00Z",
    "expires_at": "2026-10-21T12:00:00Z",
    "url": "/renders/demo-001"
  }
]
```

`expires_at` is the earlier of seven days after creation or 59 minutes after the first successful full download. The render becomes unavailable then; cleanup may delete its files later. Listing does not change retention. Returns `500` if storage cannot be read.

## GET /renders/{id}

Download the completed MP4 using the URL from the `complete` event.

- `200 OK`: `video/mp4` attachment.
- Range requests are supported (`206 Partial Content`).
- `404 Not Found`: render missing, incomplete, or expired.

```sh
curl --fail --output demo-001.mp4 http://localhost:8080/renders/demo-001
```

## Retention

Renders expire seven days after creation. A successful full, non-range GET makes a render eligible for deletion after 59 minutes; cleanup runs every minute. HEAD, partial, failed, or interrupted downloads do not shorten retention. An ID remains occupied until its render is deleted.
