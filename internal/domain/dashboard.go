package domain

import "time"

// Dashboard is a real-time snapshot of the user's financial status.
// It must never feed back into ledger or simulation logic — it is read-only analytics output.
// All fields are computed on demand; none are persisted.
//
// Computation contract:
//   - CurrentBalance      → sourced from BalanceService (authoritative ledger value)
//   - TodaySpend          → sum of transactions for current calendar day in user's timezone
//   - MonthToDateSpend    → sum of transactions from month start through today in user's timezone
//   - ProjectedEndBalance → delegated to ProjectionEngine for current month window
//   - DailySafeSpend      → derived: ProjectedEndBalance / DaysLeftInMonth (never stored)
//   - GeneratedAt         → UTC timestamp of when this snapshot was computed; used for caching
type Dashboard struct {
	CurrentBalance      int       `json:"current_balance"`
	TodaySpend          int       `json:"today_spend"`
	MonthToDateSpend    int       `json:"month_to_date_spend"`
	ProjectedEndBalance int       `json:"projected_end_balance"`
	IsDeficit           bool      `json:"is_deficit"`
	DeficitAmount       int       `json:"deficit_amount,omitempty"` // only set when IsDeficit == true
	DailySafeSpend      int       `json:"daily_safe_spend"`         // computed, never stored; negative is valid
	GeneratedAt         time.Time `json:"generated_at"`             // UTC; anchor for cache validity
}
