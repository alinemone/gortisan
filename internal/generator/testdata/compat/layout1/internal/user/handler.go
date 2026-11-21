package user

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// Handler serves the user endpoints, mounted in routes.go. A handler
// binds and validates its request (dto.go), calls the service and writes a
// response; `./gortisan make:handler user/<action>` adds one.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List serves GET /users (mounted in routes.go).
//
//	@Summary	List user
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Success	200	{object}	ListResponse
//	@Router		/users [get]
func (h *Handler) List(c *echo.Context) error {
	ctx := c.Request().Context()
	var req ListRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if err := req.Validate(ctx); err != nil {
		return err
	}

	// Call the service and write ListResponse, e.g.
	//
	//	result, err := h.service.Find(ctx, req.ID)
	//	if errors.Is(err, ErrNotFound) {
	//		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	//	}
	//	return c.JSON(http.StatusOK, ListResponse{...})
	return echo.NewHTTPError(http.StatusNotImplemented, "not implemented")
}
