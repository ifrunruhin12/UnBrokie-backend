# Repository Tests

## Prerequisites

These tests require a running PostgreSQL database. The tests will use the connection string from the `TEST_DATABASE_URL` environment variable, or default to:

```
postgres://postgres:postgres@localhost:5432/money_manager_test?sslmode=disable
```

## Running the Tests

1. Start the PostgreSQL database:
   ```bash
   docker-compose up -d
   ```

2. Create the test database (if it doesn't exist):
   ```bash
   docker-compose exec db psql -U postgres -c "CREATE DATABASE money_manager_test;"
   ```

3. Run migrations on the test database:
   ```bash
   TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/money_manager_test?sslmode=disable" go run cmd/api/main.go
   ```
   
   Or manually apply migrations from the `migrations/` directory.

4. Run the repository tests:
   ```bash
   go test ./internal/repository/... -v
   ```

## Test Coverage

Each repository has a smoke test that:
- Inserts a record
- Reads it back
- Verifies the data matches

Repositories tested:
- UserRepository
- AccountRepository
- CategoryRepository
- TransactionRepository
- RuleRepository (FixedRule)
- ConsumableRepository
- BigBuyRepository

## Notes

- Tests use `truncateTables()` to clean up between test runs
- Each test creates its own test user and related entities
- Tests are table-driven for easy extension
