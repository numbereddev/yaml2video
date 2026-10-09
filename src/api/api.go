package api

import (
	"encoding/json"
	"log"
	"net/http"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /render", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Test string `json:"test"`
		}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&body); err != nil {
			panic(err)
		}

		log.Printf("Input: %v", body)

		panic("Not implemented")
		// if _, err := w.Write([]byte("Hello, World!")); err != nil {
		// 	panic(err)
		// }
	})

	return Chain(mux, Recover)
}
