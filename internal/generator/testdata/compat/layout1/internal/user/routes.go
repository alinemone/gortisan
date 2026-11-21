package user

import "github.com/labstack/echo/v5"

// RegisterRoutes mounts the user endpoints on api, the BASE_PATH group.
// Paths are relative to it: "/users" serves BASE_PATH/users.
// `./gortisan make:handler` adds a line for each new action.
func RegisterRoutes(api *echo.Group, h *Handler) {
	g := api.Group("/users")
	g.GET("", h.List)
}
