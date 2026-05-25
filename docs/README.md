# Money Manager API Documentation

## Swagger UI

The Money Manager API includes interactive Swagger UI documentation that allows you to explore and test all API endpoints.

### Accessing Swagger UI

#### Local Development
```
http://localhost:8080/swagger
```

#### Docker
```
http://localhost:8080/swagger
```

### Features

- 📖 **Interactive Documentation** - Browse all API endpoints with detailed descriptions
- 🧪 **Try It Out** - Test API endpoints directly from the browser
- 🔐 **Authentication** - Supports JWT Bearer token authentication
- 📋 **Request/Response Examples** - See example payloads for all endpoints
- 💾 **Persistent Authorization** - Your JWT token is saved in the browser

### Quick Start

1. **Start the server**
   ```bash
   docker-compose up
   ```

2. **Open Swagger UI**
   Navigate to http://localhost:8080/swagger in your browser

3. **Register a user**
   - Click on `POST /auth/register`
   - Click "Try it out"
   - Enter email and password
   - Click "Execute"
   - Copy the JWT token from the response

4. **Authorize**
   - Click the "Authorize" button at the top
   - Enter: `Bearer <your-token>`
   - Click "Authorize"

5. **Test endpoints**
   All protected endpoints will now use your JWT token automatically!

### API Endpoints

#### Authentication
- `POST /auth/register` - Register a new user
- `POST /auth/login` - Login with email and password

#### Account
- `GET /balance` - Get current balance
- `PATCH /account/balance` - Set current balance (adjusts starting balance to match)
- `PATCH /account/timezone` - Update timezone
- `POST /account/reconcile` - Force balance reconciliation

#### Transactions
- `POST /transactions` - Create a manual transaction
- `GET /transactions` - List transactions by date range
- `PATCH /transactions/{id}/override` - Override a transaction
- `PATCH /transactions/{id}/skip` - Skip a transaction
- `PATCH /transactions/{id}/restore` - Restore a skipped transaction
- `GET /transactions/{id}/history` - Get transaction history

#### Categories
- `POST /categories` - Create a category
- `GET /categories` - List all categories
- `PATCH /categories/{id}` - Update a category
- `DELETE /categories/{id}` - Delete a category

#### Big Buys
- `POST /big-buys` - Create a big buy
- `GET /big-buys` - List big buys by month
- `PATCH /big-buys/{id}` - Update a big buy
- `DELETE /big-buys/{id}` - Delete a big buy

### OpenAPI Specification

The raw OpenAPI 3.0 specification is available at:
```
http://localhost:8080/swagger.yaml
```

You can import this file into:
- Postman
- Insomnia
- OpenAPI Generator (for client generation)
- Any OpenAPI-compatible tool

### Generating API Clients

You can generate API clients in various languages using the OpenAPI Generator:

```bash
# Install OpenAPI Generator
npm install -g @openapitools/openapi-generator-cli

# Generate TypeScript client
openapi-generator-cli generate \
  -i http://localhost:8080/swagger.yaml \
  -g typescript-axios \
  -o ./client

# Generate Python client
openapi-generator-cli generate \
  -i http://localhost:8080/swagger.yaml \
  -g python \
  -o ./client-python
```

### Notes

- All monetary amounts are in the smallest currency unit (e.g., cents for USD, paisa for BDT)
- All timestamps are in UTC (ISO 8601 format)
- JWT tokens expire after 24 hours
- Rate limiting applies to `POST /transactions` (configurable via `RATE_LIMIT_RPM` env var)
