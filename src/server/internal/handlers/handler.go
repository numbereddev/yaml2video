package handlers

import "github.com/ondics/yaml2video/src/server/internal/controller"

// Handler translates HTTP requests and responses to controller operations.
type Handler struct {
	Controller *controller.Controller
}
