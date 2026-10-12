# Internal render server API

> **Implementation status:** This document specifies the target server contract. The current `src/api/api.go` handler is a stub: it accepts `name` and Go `[]byte` YAML fields, but does not yet implement the storage policy, ID alias, validation/status behavior, progress events, cancellation, downloads, or cleanup described below. The serve Compose configuration provisions the volume and environment variable; it does not implement those behaviors. The examples below are for a server implementing this contract.

## Trust boundary and deployment

This API has **no authentication** and is for **trusted, internal-only clients**. Do not publish it on the internet or an untrusted network. YAML and media are trusted inputs; loopback binding is not a substitute for authentication if a proxy exposes the service elsewhere.

Start from the repository root:

```sh
make up
```

`docker-compose.serve.yaml` binds `127.0.0.1:8080:8080`, sets `YAML2VIDEO_DATA_DIR=/var/lib/yaml2video`, and mounts the named volume `yaml2video-data` at that path. Compose prefixes the actual volume name with its project name. Keep the loopback binding; any reverse proxy must also remain private and must not buffer SSE responses.

```sh
make down
```

Normal shutdown preserves the volume. Using `make down-volumes` deletes the stored data, including persisted lifecycle metadata.

## Storage and media

`YAML2VIDEO_DATA_DIR` overrides the platform default:

| Platform | Default data directory |
| --- | --- |
| Linux | `/var/lib/yaml2video` |
| macOS | `/Library/Application Support/yaml2video` |
| Windows | `%ProgramData%\yaml2video`; fallback `C:\ProgramData\yaml2video` if `ProgramData` is unset |

The server needs write permission to its data directory. Job data and lifecycle metadata persist there across server/container restarts when the directory is backed by persistent storage.

Relative media references in YAML resolve against `<dataDir>/media`, not the client's working directory or the container's `/data` directory. For example, `assets/image.png` resolves to `/var/lib/yaml2video/media/assets/image.png` in the serve container. The caller must provision these files or explicitly mount a media directory. There is **no media-upload endpoint and no remote media fetch**.

For the included simple example, provision `examples/simple/assets` as `<dataDir>/media/assets`. Alternatively, use a local Compose override with this additional service mount (the host source path below is relative to the repository root):

```yaml
services:
    yaml2video:
        volumes:
            - ./examples/simple:/var/lib/yaml2video/media:ro
```

Save the override as `compose.media.yaml` and launch with both configurations:

```sh
docker compose -f docker-compose.serve.yaml -f compose.media.yaml up --build -d
```

The read-only media mount supplements, rather than replaces, the writable named volume at `/var/lib/yaml2video`.

## POST /render

Send `Content-Type: application/json` with a job identifier in `name` or `id`, and YAML bytes encoded as base64 strings in `video` and `template`:

```json
{
  "name": "demo-001",
  "video": "<base64-encoded video YAML>",
  "template": "<base64-encoded template YAML>"
}
```

`id` is an alternative to `name`; use one identifier field per request. Base64 encoding preserves compatibility with the existing Go `[]byte` JSON fields. Send the actual YAML file contents, not filenames, raw YAML strings, or URLs.

- **400 Bad Request:** invalid request, including invalid payload or YAML.
- **409 Conflict:** the same ID is already occupied. Choose another ID or wait until the stored job is deleted; marking it freeable does not immediately release it.
- **200 OK:** accepted request, returned as `text/event-stream`. After streaming starts, render failures are reported by an SSE `error` event rather than a new HTTP status.

The response remains open while rendering. **SSE disconnection cancels the render**, so clients must keep reading until completion or failure. Do not automatically retry an occupied ID.

### SSE events

Each named event has JSON in its `data` field, terminated by a blank line:

| Event | JSON fields | Meaning |
| --- | --- | --- |
| `progress` | `id`, `stage`, `percent`, plus render details | Current render stage and percentage progress; additional details describe the render. |
| `complete` | `id`, `url` | Rendering succeeded. `url` is relative, `/output/{id}`. |
| `error` | `id`, `error` | Rendering failed; `error` describes the failure. |

Illustrative stream (stage names and additional render details are not fixed by this example):

```text
event: progress
data: {"id":"demo-001","stage":"render","percent":50}

: heartbeat

event: complete
data: {"id":"demo-001","url":"/output/demo-001"}

```

A failure instead emits, for example:

```text
event: error
data: {"id":"demo-001","error":"render failed"}

```

Heartbeats are SSE **comments** (lines beginning with `:`), not named events or JSON payloads. Ignore comments while keeping the connection alive. Clients should tolerate additional progress fields and use the returned completion URL relative to the server's origin.

## GET /output/{id}

Download a completed render using the relative URL from the `complete` event. The response serves the MP4 as an attachment (`Content-Type: video/mp4` and `Content-Disposition: attachment`).

A **successful full, non-range GET** marks the job freeable. HEAD requests, partial/range downloads, and failed or interrupted transfers do **not** mark it freeable. A range request must not be used when the client intends to acknowledge a full download, even if it requests all bytes.

## Retention and cleanup

- Every job expires **seven days after job creation**, not seven days after completion, download, or restart.
- A job marked freeable by a successful full download is deleted **within one hour** of that download, potentially sooner. Do not depend on the URL remaining available for the whole hour.
- Cleanup runs **every minute**.
- Creation time and freeable/lifecycle metadata persist across restarts, so restarting the server does not reset expiry or download cleanup eligibility.
- An occupied ID remains occupied until the job is deleted.

## curl example

Run from the repository root after provisioning media as described above. Python generates JSON by base64-encoding the YAML files, avoiding shell quoting problems:

```sh
python3 - <<'PY' > render-request.json
import base64
import json
from pathlib import Path

payload = {"name": "demo-001"}
for key, path in {
    "video": "examples/simple/video-simple.yaml",
    "template": "examples/simple/template-simple.yaml",
}.items():
    payload[key] = base64.b64encode(Path(path).read_bytes()).decode("ascii")
print(json.dumps(payload))
PY

curl --fail-with-body --no-buffer \
  -H 'Content-Type: application/json' \
  -H 'Accept: text/event-stream' \
  --data-binary @render-request.json \
  http://127.0.0.1:8080/render
```

Wait for `complete`, then use its returned URL (shown here for `demo-001`):

```sh
curl --fail --output demo-001.mp4 \
  http://127.0.0.1:8080/output/demo-001
```

This full download marks the job freeable. Stopping the first curl while rendering cancels the render. HTTP errors cause curl to fail; an SSE `error` still arrives over HTTP 200, so clients must inspect the event stream rather than rely solely on curl's exit status.
