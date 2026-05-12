package domain

import (
	"encoding/json"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// **Property 38: JSON Round-Trip for Domain Objects**
// **Validates: Requirements 12.3, 13.5**
//
// For any valid domain object (Transaction, FixedRule, ConsumableRule, BigBuy, Category, Account, User, Projection, Dashboard),
// serializing it to JSON and deserializing it back should produce an object with identical field values.

// genString generates non-empty strings for required fields
func genString() *rapid.Generator[string] {
	return rapid.StringMatching(`^[a-zA-Z0-9_\-@.]+$`).Filter(func(s string) bool {
		return len(s) > 0 && len(s) <= 100
	})
}

// genEmail generates valid email addresses
func genEmail() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		local := rapid.StringMatching(`^[a-zA-Z0-9_\-]+$`).Filter(func(s string) bool {
			return len(s) > 0 && len(s) <= 50
		}).Draw(t, "local")
		domain := rapid.StringMatching(`^[a-zA-Z0-9\-]+$`).Filter(func(s string) bool {
			return len(s) > 0 && len(s) <= 50
		}).Draw(t, "domain")
		tld := rapid.SampledFrom([]string{"com", "org", "net", "io", "dev"}).Draw(t, "tld")
		return local + "@" + domain + "." + tld
	})
}

// genTime generates valid timestamps
func genTime() *rapid.Generator[time.Time] {
	return rapid.Custom(func(t *rapid.T) time.Time {
		// Generate timestamps between 2020-01-01 and 2030-12-31
		min := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
		max := time.Date(2030, 12, 31, 23, 59, 59, 0, time.UTC).Unix()
		ts := rapid.Int64Range(min, max).Draw(t, "timestamp")
		return time.Unix(ts, 0).UTC()
	})
}

// genOptionalTime generates optional timestamps
func genOptionalTime() *rapid.Generator[*time.Time] {
	return rapid.Custom(func(t *rapid.T) *time.Time {
		if rapid.Bool().Draw(t, "hasTime") {
			ts := genTime().Draw(t, "time")
			return &ts
		}
		return nil
	})
}

// genOptionalString generates optional strings
func genOptionalString() *rapid.Generator[*string] {
	return rapid.Custom(func(t *rapid.T) *string {
		if rapid.Bool().Draw(t, "hasString") {
			s := genString().Draw(t, "string")
			return &s
		}
		return nil
	})
}

// genTransactionType generates valid transaction types
func genTransactionType() *rapid.Generator[TransactionType] {
	return rapid.SampledFrom([]TransactionType{
		TransactionTypeRuleGenerated,
		TransactionTypeManual,
		TransactionTypeOverride,
	})
}

// genSourceType generates valid source types
func genSourceType() *rapid.Generator[SourceType] {
	return rapid.SampledFrom([]SourceType{
		SourceTypeRule,
		SourceTypeConsumable,
		SourceTypeTransaction,
	})
}

// genOptionalSourceType generates optional source types
func genOptionalSourceType() *rapid.Generator[*SourceType] {
	return rapid.Custom(func(t *rapid.T) *SourceType {
		if rapid.Bool().Draw(t, "hasSourceType") {
			st := genSourceType().Draw(t, "sourceType")
			return &st
		}
		return nil
	})
}

// genFrequency generates valid frequency values
func genFrequency() *rapid.Generator[Frequency] {
	return rapid.SampledFrom([]Frequency{
		FrequencyDaily,
		FrequencyWeekday,
		FrequencyWeekend,
	})
}

// genTransaction generates arbitrary Transaction instances
func genTransaction() *rapid.Generator[Transaction] {
	return rapid.Custom(func(t *rapid.T) Transaction {
		return Transaction{
			ID:             genString().Draw(t, "id"),
			UserID:         genString().Draw(t, "userID"),
			Type:           genTransactionType().Draw(t, "type"),
			CategoryID:     genString().Draw(t, "categoryID"),
			Amount:         rapid.IntRange(-1000000, 1000000).Filter(func(n int) bool { return n != 0 }).Draw(t, "amount"),
			IsSkipped:      rapid.Bool().Draw(t, "isSkipped"),
			IsOverridden:   rapid.Bool().Draw(t, "isOverridden"),
			SourceID:       genOptionalString().Draw(t, "sourceID"),
			SourceType:     genOptionalSourceType().Draw(t, "sourceType"),
			Note:           rapid.String().Draw(t, "note"),
			Date:           genTime().Draw(t, "date"),
			GenerationDate: genOptionalTime().Draw(t, "generationDate"),
			UpdatedAt:      genTime().Draw(t, "updatedAt"),
			DeletedAt:      genOptionalTime().Draw(t, "deletedAt"),
			CreatedAt:      genTime().Draw(t, "createdAt"),
		}
	})
}

// genFixedRule generates arbitrary FixedRule instances
func genFixedRule() *rapid.Generator[FixedRule] {
	return rapid.Custom(func(t *rapid.T) FixedRule {
		return FixedRule{
			ID:         genString().Draw(t, "id"),
			UserID:     genString().Draw(t, "userID"),
			Name:       genString().Draw(t, "name"),
			CategoryID: genString().Draw(t, "categoryID"),
			Amount:     rapid.IntRange(1, 1000000).Draw(t, "amount"),
			Frequency:  genFrequency().Draw(t, "frequency"),
			IsActive:   rapid.Bool().Draw(t, "isActive"),
			CreatedAt:  genTime().Draw(t, "createdAt"),
		}
	})
}

// genConsumableRule generates arbitrary ConsumableRule instances
func genConsumableRule() *rapid.Generator[ConsumableRule] {
	return rapid.Custom(func(t *rapid.T) ConsumableRule {
		return ConsumableRule{
			ID:               genString().Draw(t, "id"),
			UserID:           genString().Draw(t, "userID"),
			Name:             genString().Draw(t, "name"),
			Stock:            rapid.IntRange(0, 10000).Draw(t, "stock"),
			UsagePerDay:      rapid.IntRange(1, 100).Draw(t, "usagePerDay"),
			RestockAmount:    rapid.IntRange(1, 1000).Draw(t, "restockAmount"),
			RestockCost:      rapid.IntRange(1, 100000).Draw(t, "restockCost"),
			RestockThreshold: rapid.IntRange(0, 100).Draw(t, "restockThreshold"),
			IsDepleted:       rapid.Bool().Draw(t, "isDepleted"),
			LastRestockDate:  genOptionalTime().Draw(t, "lastRestockDate"),
			CreatedAt:        genTime().Draw(t, "createdAt"),
		}
	})
}

// genBigBuy generates arbitrary BigBuy instances
func genBigBuy() *rapid.Generator[BigBuy] {
	return rapid.Custom(func(t *rapid.T) BigBuy {
		return BigBuy{
			ID:         genString().Draw(t, "id"),
			UserID:     genString().Draw(t, "userID"),
			Title:      genString().Draw(t, "title"),
			Amount:     rapid.IntRange(-1000000, -1).Draw(t, "amount"),
			CategoryID: genString().Draw(t, "categoryID"),
			Note:       rapid.String().Draw(t, "note"),
			Date:       genTime().Draw(t, "date"),
			DeletedAt:  genOptionalTime().Draw(t, "deletedAt"),
			CreatedAt:  genTime().Draw(t, "createdAt"),
		}
	})
}

// genCategory generates arbitrary Category instances
func genCategory() *rapid.Generator[Category] {
	return rapid.Custom(func(t *rapid.T) Category {
		return Category{
			ID:        genString().Draw(t, "id"),
			UserID:    genString().Draw(t, "userID"),
			Name:      genString().Draw(t, "name"),
			CreatedAt: genTime().Draw(t, "createdAt"),
		}
	})
}

// genAccount generates arbitrary Account instances
func genAccount() *rapid.Generator[Account] {
	return rapid.Custom(func(t *rapid.T) Account {
		return Account{
			ID:               genString().Draw(t, "id"),
			UserID:           genString().Draw(t, "userID"),
			StartingBalance:  rapid.IntRange(-1000000, 1000000).Draw(t, "startingBalance"),
			CurrentBalance:   rapid.IntRange(-1000000, 1000000).Draw(t, "currentBalance"),
			BalanceDirty:     rapid.Bool().Draw(t, "balanceDirty"),
			LastReconciledAt: genOptionalTime().Draw(t, "lastReconciledAt"),
			Currency:         rapid.SampledFrom([]string{"BDT", "USD", "EUR", "GBP"}).Draw(t, "currency"),
			Timezone:         rapid.SampledFrom([]string{"UTC", "Asia/Dhaka", "America/New_York", "Europe/London"}).Draw(t, "timezone"),
			CreatedAt:        genTime().Draw(t, "createdAt"),
		}
	})
}

// genUser generates arbitrary User instances
func genUser() *rapid.Generator[User] {
	return rapid.Custom(func(t *rapid.T) User {
		return User{
			ID:           genString().Draw(t, "id"),
			Email:        genEmail().Draw(t, "email"),
			PasswordHash: genString().Draw(t, "passwordHash"),
			CreatedAt:    genTime().Draw(t, "createdAt"),
			UpdatedAt:    genTime().Draw(t, "updatedAt"),
		}
	})
}

// genProjection generates arbitrary Projection instances
func genProjection() *rapid.Generator[Projection] {
	return rapid.Custom(func(t *rapid.T) Projection {
		projectedEndBalance := rapid.IntRange(-1000000, 1000000).Draw(t, "projectedEndBalance")
		isDeficit := projectedEndBalance < 0
		deficitAmount := 0
		if isDeficit {
			deficitAmount = -projectedEndBalance
		}
		return Projection{
			From:                  genTime().Draw(t, "from"),
			To:                    genTime().Draw(t, "to"),
			CurrentBalance:        rapid.IntRange(-1000000, 1000000).Draw(t, "currentBalance"),
			FuturePlannedExpenses: rapid.IntRange(0, 1000000).Draw(t, "futurePlannedExpenses"),
			ProjectedEndBalance:   projectedEndBalance,
			IsDeficit:             isDeficit,
			DeficitAmount:         deficitAmount,
		}
	})
}

// genDashboard generates arbitrary Dashboard instances
func genDashboard() *rapid.Generator[Dashboard] {
	return rapid.Custom(func(t *rapid.T) Dashboard {
		projectedEndBalance := rapid.IntRange(-1000000, 1000000).Draw(t, "projectedEndBalance")
		isDeficit := projectedEndBalance < 0
		deficitAmount := 0
		if isDeficit {
			deficitAmount = -projectedEndBalance
		}
		return Dashboard{
			CurrentBalance:      rapid.IntRange(-1000000, 1000000).Draw(t, "currentBalance"),
			TodaySpend:          rapid.IntRange(-100000, 100000).Draw(t, "todaySpend"),
			MonthToDateSpend:    rapid.IntRange(-1000000, 1000000).Draw(t, "monthToDateSpend"),
			ProjectedEndBalance: projectedEndBalance,
			IsDeficit:           isDeficit,
			DeficitAmount:       deficitAmount,
			DailySafeSpend:      rapid.IntRange(-100000, 100000).Draw(t, "dailySafeSpend"),
			GeneratedAt:         genTime().Draw(t, "generatedAt"),
		}
	})
}

// TestTransactionJSONRoundTrip tests JSON serialization round-trip for Transaction
func TestTransactionJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genTransaction().Draw(t, "transaction")

		// Marshal to JSON
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		// Unmarshal back
		var decoded Transaction
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		// Compare - use time.Equal for time.Time fields
		if original.ID != decoded.ID ||
			original.UserID != decoded.UserID ||
			original.Type != decoded.Type ||
			original.CategoryID != decoded.CategoryID ||
			original.Amount != decoded.Amount ||
			original.IsSkipped != decoded.IsSkipped ||
			original.IsOverridden != decoded.IsOverridden ||
			!equalOptionalString(original.SourceID, decoded.SourceID) ||
			!equalOptionalSourceType(original.SourceType, decoded.SourceType) ||
			original.Note != decoded.Note ||
			!original.Date.Equal(decoded.Date) ||
			!equalOptionalTime(original.GenerationDate, decoded.GenerationDate) ||
			!original.UpdatedAt.Equal(decoded.UpdatedAt) ||
			!equalOptionalTime(original.DeletedAt, decoded.DeletedAt) ||
			!original.CreatedAt.Equal(decoded.CreatedAt) {
			t.Fatalf("round-trip mismatch:\noriginal: %+v\ndecoded:  %+v", original, decoded)
		}
	})
}

// TestFixedRuleJSONRoundTrip tests JSON serialization round-trip for FixedRule
func TestFixedRuleJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genFixedRule().Draw(t, "fixedRule")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var decoded FixedRule
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if original.ID != decoded.ID ||
			original.UserID != decoded.UserID ||
			original.Name != decoded.Name ||
			original.CategoryID != decoded.CategoryID ||
			original.Amount != decoded.Amount ||
			original.Frequency != decoded.Frequency ||
			original.IsActive != decoded.IsActive ||
			!original.CreatedAt.Equal(decoded.CreatedAt) {
			t.Fatalf("round-trip mismatch:\noriginal: %+v\ndecoded:  %+v", original, decoded)
		}
	})
}

// TestConsumableRuleJSONRoundTrip tests JSON serialization round-trip for ConsumableRule
func TestConsumableRuleJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genConsumableRule().Draw(t, "consumableRule")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var decoded ConsumableRule
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if original.ID != decoded.ID ||
			original.UserID != decoded.UserID ||
			original.Name != decoded.Name ||
			original.Stock != decoded.Stock ||
			original.UsagePerDay != decoded.UsagePerDay ||
			original.RestockAmount != decoded.RestockAmount ||
			original.RestockCost != decoded.RestockCost ||
			original.RestockThreshold != decoded.RestockThreshold ||
			original.IsDepleted != decoded.IsDepleted ||
			!equalOptionalTime(original.LastRestockDate, decoded.LastRestockDate) ||
			!original.CreatedAt.Equal(decoded.CreatedAt) {
			t.Fatalf("round-trip mismatch:\noriginal: %+v\ndecoded:  %+v", original, decoded)
		}
	})
}

// TestBigBuyJSONRoundTrip tests JSON serialization round-trip for BigBuy
func TestBigBuyJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genBigBuy().Draw(t, "bigBuy")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var decoded BigBuy
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if original.ID != decoded.ID ||
			original.UserID != decoded.UserID ||
			original.Title != decoded.Title ||
			original.Amount != decoded.Amount ||
			original.CategoryID != decoded.CategoryID ||
			original.Note != decoded.Note ||
			!original.Date.Equal(decoded.Date) ||
			!equalOptionalTime(original.DeletedAt, decoded.DeletedAt) ||
			!original.CreatedAt.Equal(decoded.CreatedAt) {
			t.Fatalf("round-trip mismatch:\noriginal: %+v\ndecoded:  %+v", original, decoded)
		}
	})
}

// TestCategoryJSONRoundTrip tests JSON serialization round-trip for Category
func TestCategoryJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genCategory().Draw(t, "category")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var decoded Category
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if original.ID != decoded.ID ||
			original.UserID != decoded.UserID ||
			original.Name != decoded.Name ||
			!original.CreatedAt.Equal(decoded.CreatedAt) {
			t.Fatalf("round-trip mismatch:\noriginal: %+v\ndecoded:  %+v", original, decoded)
		}
	})
}

// TestAccountJSONRoundTrip tests JSON serialization round-trip for Account
func TestAccountJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genAccount().Draw(t, "account")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var decoded Account
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if original.ID != decoded.ID ||
			original.UserID != decoded.UserID ||
			original.StartingBalance != decoded.StartingBalance ||
			original.CurrentBalance != decoded.CurrentBalance ||
			original.BalanceDirty != decoded.BalanceDirty ||
			!equalOptionalTime(original.LastReconciledAt, decoded.LastReconciledAt) ||
			original.Currency != decoded.Currency ||
			original.Timezone != decoded.Timezone ||
			!original.CreatedAt.Equal(decoded.CreatedAt) {
			t.Fatalf("round-trip mismatch:\noriginal: %+v\ndecoded:  %+v", original, decoded)
		}
	})
}

// TestUserJSONRoundTrip tests JSON serialization round-trip for User
func TestUserJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genUser().Draw(t, "user")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var decoded User
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if original.ID != decoded.ID ||
			original.Email != decoded.Email ||
			original.PasswordHash != decoded.PasswordHash ||
			!original.CreatedAt.Equal(decoded.CreatedAt) ||
			!original.UpdatedAt.Equal(decoded.UpdatedAt) {
			t.Fatalf("round-trip mismatch:\noriginal: %+v\ndecoded:  %+v", original, decoded)
		}
	})
}

// TestProjectionJSONRoundTrip tests JSON serialization round-trip for Projection
func TestProjectionJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genProjection().Draw(t, "projection")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var decoded Projection
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if !original.From.Equal(decoded.From) ||
			!original.To.Equal(decoded.To) ||
			original.CurrentBalance != decoded.CurrentBalance ||
			original.FuturePlannedExpenses != decoded.FuturePlannedExpenses ||
			original.ProjectedEndBalance != decoded.ProjectedEndBalance ||
			original.IsDeficit != decoded.IsDeficit ||
			original.DeficitAmount != decoded.DeficitAmount {
			t.Fatalf("round-trip mismatch:\noriginal: %+v\ndecoded:  %+v", original, decoded)
		}
	})
}

// TestDashboardJSONRoundTrip tests JSON serialization round-trip for Dashboard
func TestDashboardJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genDashboard().Draw(t, "dashboard")

		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var decoded Dashboard
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if original.CurrentBalance != decoded.CurrentBalance ||
			original.TodaySpend != decoded.TodaySpend ||
			original.MonthToDateSpend != decoded.MonthToDateSpend ||
			original.ProjectedEndBalance != decoded.ProjectedEndBalance ||
			original.IsDeficit != decoded.IsDeficit ||
			original.DeficitAmount != decoded.DeficitAmount ||
			original.DailySafeSpend != decoded.DailySafeSpend ||
			!original.GeneratedAt.Equal(decoded.GeneratedAt) {
			t.Fatalf("round-trip mismatch:\noriginal: %+v\ndecoded:  %+v", original, decoded)
		}
	})
}

// Helper functions for comparing optional fields

func equalOptionalString(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalOptionalSourceType(a, b *SourceType) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func equalOptionalTime(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}
