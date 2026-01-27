package pomomo

import "time"

type ExistingRecord[T ~string] struct {
	ID        T
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewExistingRecord[T ~string](id string) ExistingRecord[T] {
	now := time.Now()
	return ExistingRecord[T]{
		ID:        T(id),
		CreatedAt: now,
		UpdatedAt: now,
	}
}
