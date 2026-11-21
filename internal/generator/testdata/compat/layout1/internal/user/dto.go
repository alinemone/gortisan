package user

import (
	"context"

	"example.com/compat/internal/server/validate"
)

// The requests and responses of the user endpoints. A request is bound
// from the path (`param:"id"`), the query string (`query:"page"`) and the
// JSON body (`json:"name"`), then validated; a response is built from the
// entity here, so a column change does not change the API by accident.
// `./gortisan make:handler` adds a pair for each action.

// ListRequest is the input of List. c.Bind fills it from the
// path (`param:"id"`), the query string (`query:"page"`) and the JSON body
// (`json:"name"`).
type ListRequest struct{}

// Validate checks the request after binding and returns a 422 listing every
// invalid field, or nil. Messages are keys of trans/lang/fa.json.
func (r *ListRequest) Validate(ctx context.Context) error {
	var errs validate.Errors
	// if r.ID <= 0 {
	// 	errs.Add("path.id", "general:field-required", "شناسه")
	// }
	// if err := errs.Check(ctx, address.NewMobile("body.mobile", r.Mobile)); err != nil {
	// 	return err
	// }
	return errs.Err()
}

// ListResponse is the output of List, written with c.JSON.
// Build it from the entity here, so a column change does not change the API
// by accident.
type ListResponse struct{}
