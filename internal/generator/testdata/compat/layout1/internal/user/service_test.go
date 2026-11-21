package user

import (
	"context"
	"errors"
	"testing"
)

// fakeRepository is an in-memory Repository, so the service is tested
// without a database. It embeds Repository: a method it does not implement
// panics when called — implement the ones your tests need.
type fakeRepository struct {
	Repository
	rows   map[int64]*User
	nextID int64
}

func newFakeRepository(rows ...*User) *fakeRepository {
	f := &fakeRepository{rows: map[int64]*User{}}
	for _, m := range rows {
		f.nextID++
		m.ID = f.nextID
		f.rows[m.ID] = m
	}
	return f
}

func (f *fakeRepository) Find(ctx context.Context, id int64) (*User, error) {
	m, ok := f.rows[id]
	if !ok {
		return nil, ErrNotFound
	}
	return m, nil
}

func TestServiceFind(t *testing.T) {
	stored := &User{}
	s := NewService(newFakeRepository(stored))

	tests := []struct {
		name    string
		id      int64
		want    *User
		wantErr error
	}{
		{name: "found", id: stored.ID, want: stored},
		{name: "missing", id: 99, wantErr: ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Find(context.Background(), tt.id)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Fatalf("Find(%d) = %v, %v; want %v, %v", tt.id, got, err, tt.want, tt.wantErr)
			}
		})
	}
}
