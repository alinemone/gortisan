package user

import "context"

// Repository is what Service needs from storage. It is declared here, by the
// code that uses it; Postgres (postgres.go) implements it, and a test can give
// Service a fake one and run without a database. Add a method when the
// service needs a new query.
type Repository interface {
	Find(ctx context.Context, id int64) (*User, error)
	List(ctx context.Context, limit, offset int) ([]*User, error)
	Create(ctx context.Context, m *User) error
	Update(ctx context.Context, m *User, columns ...string) error
	Delete(ctx context.Context, id int64) error
}

// Service holds the user use cases: the one entry point for the
// handlers, workers and other features. Methods take a context first and
// return the errors of user.go; the caller decides what an error means to
// its client.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Find returns ErrNotFound when there is no such user.
func (s *Service) Find(ctx context.Context, id int64) (*User, error) {
	return s.repo.Find(ctx, id)
}
