package server

import (
	"net/http"

	"github.com/ondics/yaml2video/src/server/internal/controller"
	"github.com/ondics/yaml2video/src/server/internal/handlers"
)

func NewRouter(backend *controller.Controller) http.Handler {
	h := &handlers.Handler{Controller: backend}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /render", h.HandleRender)
	mux.HandleFunc("GET /renders/{id}", h.HandleDownload)
	return Chain(mux, Recover)
}
