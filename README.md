# ARS Kit API

A REST API starter kit built with Go, Echo framework, and PostgreSQL.

## Tech Stack

- Go 1.23.4
- Echo v4 (HTTP framework)
- PostgreSQL (via pgx/v5)
- Zerolog (structured logging)
- Swagger (API documentation)


## Prerequisites

- Go 1.23.4 or higher
- PostgreSQL database
- Environment variables configured (see `.env`)

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd ars-kit

# Install dependencies
go mod download

# Set up database
psql -U <username> -d <database> -f database/sql/users.sql
```

## Configuration

Create a `.env` file in the project root with the following variables:

```env
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=your_database
```

## Running

```bash
# Run the application
go run src/main.go

# Server starts on localhost:8080 (or configured PORT)
```

## API Endpoints

- `GET /health` - Health check
- User endpoints (see Swagger docs)

## API Documentation

Access Swagger documentation at `/docs/api.html` when the server is running.

### Generating Swagger Documentation

To regenerate Swagger documentation after API changes:

```bash
# Install swag CLI (if not already installed)
go install github.com/swaggo/swag/cmd/swag@latest

# Generate docs from annotations in code
swag init -g src/main.go -o docs

# Generate html file
npx @redocly/cli build-docs docs/swagger.yaml -o docs/api.html
```

The generated files will be placed in the `docs/` directory.

## Testing

### Setting Up Test Database

1. Create a test database in PostgreSQL:

```sql
CREATE DATABASE go_test_your_app;
GRANT ALL PRIVILEGES ON DATABASE go_test_your_app TO postgres;
```

2. Update test configuration in `testing/config.go` if needed (defaults: localhost:5432, user: postgres, db: go_test_your_app)

3. Run tests:

```bash
# Run all tests
go test -v ./... -count=1
```

The testing suite provides database-isolated test execution with automatic schema creation and cleanup. See `testing/README.md` for detailed documentation.

## Project Structure

```
ars-kit/
├── config/          # Configuration management
├── database/        # Database connection and SQL schemas
├── docs/            # API documentation (Swagger)
├── src/
│   ├── app/         # Application modules
│   │   └── user/    # User feature
│   └── main.go      # Application entry point
└── testing/         # Test suite and helpers
```

## License

Not specified
