package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ondics/yaml2video/src/render"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /render", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name     string `json:"name"`
			Video    []byte `json:"video"`
			Template []byte `json:"template"`
		}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&body); err != nil {
			panic(err)
		}

		// Create event stream
		w.Header().Set("X-Accel-Buffering", "no")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		path := filepath.Join(".", body.Name)
		plan, err := render.LoadBytes(
			body.Video,
			body.Template,
			path,
		)
		if err != nil {
			panic(err)
		}

		if err := plan.Render(context.Background()); err != nil {
			os.RemoveAll(path)
			panic(err)
		}

		for range 5 {
			WriteMust(w, "event: echo\n")
			WriteMust(w, fmt.Sprintf("data: %s\n\n", body.Name))
			time.Sleep(500 * time.Millisecond)
		}
	})

	return Chain(mux, Recover)
}
