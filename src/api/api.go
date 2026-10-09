package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /render", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Test string `json:"test"`
		}

		// Create event stream
		w.Header().Set("X-Accel-Buffering", "no")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&body); err != nil {
			panic(err)
		}

		for range 5 {
			WriteMust(w, "event: echo\n")
			WriteMust(w, fmt.Sprintf("data: %s\n\n", body.Test))
			time.Sleep(500 * time.Millisecond)
		}
	})

	return Chain(mux, Recover)
}
