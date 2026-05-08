package domain

import "time"

// Projection is a pure forward-looking simulation result for a given time window.
// It must never feed back into ledger or simulation logic — it is read-only analytics output.
// All fields are computed on demand; none are persisted.
//
// Computation contract:
//   - CurrentBalance  → sourced from BalanceService (authoritative ledger value)
//   - FuturePlannedExpenses → simulated by ProjectionEngine from rules + big buys in [From, To]
//   - ProjectedEndBalance  → CurrentBalance - FuturePlannedExpenses
//   - IsDeficit / DeficitAmount → derived from ProjectedEndBalance < 0
type Projection struct {
	From                  time.Time `json:"from"`                    // start of projection window (inclusive)
	To                    time.Time `json:"to"`                      // end of projection window (inclusive)
	CurrentBalance        int       `json:"current_balance"`
	FuturePlannedExpenses int       `json:"future_planned_expenses"` // always non-negative magnitude
	ProjectedEndBalance   int       `json:"projected_end_balance"`
	IsDeficit             bool      `json:"is_deficit"`
	DeficitAmount         int       `json:"deficit_amount,omitempty"` // only set when IsDeficit == true
}
