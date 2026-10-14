FROM golang:1.27.1-bookworm

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates ffmpeg fontconfig fonts-dejavu-core \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath \
	-ldflags="-s -w -X github.com/ondics/yaml2video/src/render.SchemaDir=/usr/local/share/yaml2video" \
	-o /usr/local/bin/yaml2video ./src

COPY docs/video.schema.json docs/template.schema.json /usr/local/share/yaml2video/

WORKDIR /data
ENTRYPOINT ["yaml2video"]
