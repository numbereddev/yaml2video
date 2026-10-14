# Render API

For trusted internal clients only. No authentication.

## POST /render

Send `Content-Type: application/json`:

```json
{
  "id": "demo-001",
  "video": "<base64-encoded video YAML>",
  "template": "<base64-encoded template YAML>"
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
| `400` | Invalid request or YAML |
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
