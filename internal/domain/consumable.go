package domain

import "time"

// ConsumableRule invariants:
//   - Stock is mutable simulation state; updated atomically via SELECT FOR UPDATE in the consumable engine
//   - UsagePerDay > 0; enforced by DB CHECK constraint
//   - Stock floor is 0; IsDepleted is set true when a deduction would go negative
//   - At most one restock transaction is generated per consumable per day (idempotency via uniq_tx_source_date index)
//   - LastRestockDate is the idempotency anchor; a nil value means the consumable has never been restocked
type ConsumableRule struct {
	ID               string     `db:"id"`
	UserID           string     `db:"user_id"`
	Name             string     `db:"name"`
	Stock            int        `db:"stock"`
	UsagePerDay      int        `db:"usage_per_day"`
	RestockAmount    int        `db:"restock_amount"`
	RestockCost      int        `db:"restock_cost"`
	RestockThreshold int        `db:"restock_threshold"`
	IsDepleted       bool       `db:"is_depleted"`
	LastRestockDate  *time.Time `db:"last_restock_date"`
	CreatedAt        time.Time  `db:"created_at"`
}
