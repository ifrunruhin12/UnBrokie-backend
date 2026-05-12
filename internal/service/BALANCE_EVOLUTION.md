# Balance Service Evolution Path

## Current Architecture (MVP)

**Synchronous reconciliation on read:**
```
GetBalance() → check dirty/stale → recompute → update cache → return
```

**Characteristics:**
- ✅ Simple, correct, easy to reason about
- ✅ No background jobs needed
- ✅ Tunable via `stalenessThresh` config
- ⚠️ Slow path blocks API response (acceptable for MVP)

**Config tuning:**
- Dev: `BALANCE_STALENESS_THRESHOLD=1m` (aggressive, immediate feedback)
- Prod: `BALANCE_STALENESS_THRESHOLD=5m` (lazy, better performance)

---

## Future Architecture (Scale)

### Fast Path + Slow Path Split

**Fast Path (API):**
```go
GetBalance() → return cached balance immediately (always <50ms)
```

**Slow Path (Background Job):**
```go
ReconciliationWorker:
  - Poll for dirty accounts every 30s
  - Recompute true balance
  - Update cache atomically
  - Log mismatches
```

### Why This Matters

**Current bottleneck:**
- If 1000 users hit `/balance` simultaneously and all are stale
- All 1000 requests run `computeTrueBalance` synchronously
- Database gets hammered with SUM queries

**After split:**
- All 1000 requests return cached value instantly
- Background worker reconciles dirty accounts asynchronously
- Database load is smoothed over time

### Migration Path

**Phase 1 (current):**
```go
GetBalance() → sync reconcile if dirty/stale
```

**Phase 2 (add async worker, keep sync as fallback):**
```go
GetBalance() → return cached (even if dirty)
Worker → reconcile dirty accounts in background
```

**Phase 3 (pure async):**
```go
GetBalance() → always return cached
Worker → reconcile all accounts periodically
```

### Implementation Notes

**Worker design:**
```go
type ReconciliationWorker struct {
    balanceService BalanceService
    accountRepo    AccountRepository
    interval       time.Duration
}

func (w *ReconciliationWorker) Run(ctx context.Context) {
    ticker := time.NewTicker(w.interval)
    for {
        select {
        case <-ticker.C:
            w.reconcileDirtyAccounts(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (w *ReconciliationWorker) reconcileDirtyAccounts(ctx context.Context) {
    // SELECT id, user_id FROM accounts WHERE balance_dirty = TRUE LIMIT 100
    // For each: recompute + ReconcileBalance
}
```

**Queue-based alternative (even better):**
```
Transaction created → publish "balance_dirty" event → worker consumes → reconcile
```

### Why Current Architecture Enables This

1. **`ReconcileBalance` is already atomic** — no race conditions
2. **`stalenessThresh` already decouples policy from code** — easy to tune
3. **Service layer doesn't know about HTTP** — can be called from worker
4. **Repository interface is clean** — easy to mock for worker tests

The current design naturally evolves into the async pattern without major refactoring.

---

## Repository Improvements

### Current Issue

```go
// Service layer knows about date ranges (implementation detail leak)
minDate := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
maxDate := time.Date(2100, 12, 31, 23, 59, 59, 999999999, time.UTC)
bigBuySum, err := s.bigBuyRepo.SumByDateRange(ctx, userID, minDate, maxDate)
```

### Better Interface

```go
// Repository interface expresses intent clearly
bigBuySum, err := s.bigBuyRepo.SumAll(ctx, userID)
```

**Benefits:**
- Intent is obvious (sum ALL big buys)
- Service doesn't know about date range logic
- Repository can optimize query internally
- Easier to test (no magic dates in tests)

**Implementation:**
```go
func (r *bigBuyRepository) SumAll(ctx context.Context, userID string) (int, error) {
    var sum int
    err := r.db.QueryRow(ctx,
        `SELECT COALESCE(SUM(amount), 0)
         FROM big_buys
         WHERE user_id = $1 AND deleted_at IS NULL`,
        userID,
    ).Scan(&sum)
    return sum, err
}
```

Same for transactions — `SumNonSkippedNonOverridden` is already clean.

---

## Key Takeaway

**The current architecture is not just "good enough for MVP".**

It's **designed to evolve** into the async pattern without major refactoring.

That's scalable system design.
