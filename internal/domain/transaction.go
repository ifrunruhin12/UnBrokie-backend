package domain

import "time"

// TransactionType is the type of a transaction.
type TransactionType string

const (
	TransactionTypeRuleGenerated TransactionType = "rule_generated"
	TransactionTypeManual        TransactionType = "manual"
	TransactionTypeOverride      TransactionType = "override"
)

// SourceType indicates what entity SourceID references.
type SourceType string

const (
	SourceTypeRule       SourceType = "rule"
	SourceTypeConsumable SourceType = "consumable"
	SourceTypeTransaction SourceType = "transaction"
)

// Transaction flag invariants:
//   - IsSkipped == true  → excluded from all balance/projection calculations; may coexist with IsOverridden
//   - IsOverridden == true → a newer override transaction exists with SourceID pointing to this record's root;
//                            the original record is preserved for audit; only the override amount is used in calculations
//   - Both flags are orthogonal: a transaction can be overridden and independently skipped
type Transaction struct {
	ID             string          `db:"id"`
	UserID         string          `db:"user_id"`
	Type           TransactionType `db:"type"`
	CategoryID     string          `db:"category_id"`
	// Amount is stored in the smallest currency unit (e.g. paise). Negative = expense, positive = income/refund.
	// Future migration path: introduce a Money type at the repository boundary; DB column stays INTEGER.
	Amount         int             `db:"amount"`
	IsSkipped      bool            `db:"is_skipped"`
	IsOverridden   bool            `db:"is_overridden"`    // denormalized; set true when an override tx points to this one
	SourceID       *string         `db:"source_id"`        // nullable
	SourceType     *SourceType     `db:"source_type"`      // nullable
	Note           string          `db:"note"`
	Date           time.Time       `db:"date"`             // stored as TIMESTAMPTZ in UTC
	GenerationDate *time.Time      `db:"generation_date"`  // logical date in user TZ; idempotency key (rule_generated only)
	UpdatedAt      time.Time       `db:"updated_at"`       // set on every mutation (skip, restore, override)
	DeletedAt      *time.Time      `db:"deleted_at"`       // soft delete
	CreatedAt      time.Time       `db:"created_at"`
}
