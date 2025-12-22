package pomomo

import "time"

type DBRow struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}
