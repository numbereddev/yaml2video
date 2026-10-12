FROM golang:1.27.1-bookworm AS development

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ffmpeg fontconfig fonts-dejavu-core \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /src

FROM development AS build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath \
	-ldflags="-s -w -X github.com/ondics/yaml2video/src/render.SchemaDir=/usr/local/share/yaml2video" \
	-o /yaml2video ./src/cmd/yaml2video

FROM debian:bookworm-slim AS runtime

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates ffmpeg fontconfig fonts-dejavu-core \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=build /yaml2video /usr/local/bin/yaml2video
COPY docs/video.schema.json docs/template.schema.json /usr/local/share/yaml2video/
WORKDIR /data
ENTRYPOINT ["yaml2video"]
