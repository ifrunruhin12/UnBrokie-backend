package domain

import "time"

type BigBuy struct {
	ID         string     `db:"id"`
	UserID     string     `db:"user_id"`
	Title      string     `db:"title"`
	// Amount is stored in the smallest currency unit (e.g. paise). Always negative (expense), consistent with transactions.
	Amount     int        `db:"amount"`
	CategoryID string     `db:"category_id"`
	Note       string     `db:"note"`
	Date       time.Time  `db:"date"`
	DeletedAt  *time.Time `db:"deleted_at"` // soft delete
	CreatedAt  time.Time  `db:"created_at"`
}
