# Testing Suite Documentation

This testing suite provides database-isolated test execution for Go applications using PostgreSQL. It acts as a "main" function for your tests, initializing all necessary components (pool, storage, service) so your tests stay clean and focused.

## Features

- **Database Schema Isolation**: Each test scenario runs in its own random schema
- **Real Database Testing**: Uses actual PostgreSQL database
- **App Initialization**: Automatically sets up pool, storage, and service components
- **Fixture Support**: Built-in helpers for setting up test data with DataXxx pattern
- **State Verification**: Compare initial and after states to ensure data integrity
- **Clean API**: Follows BDD-style testing patterns with consistent structure

## Setup

### Prerequisites

1. PostgreSQL database server running
2. Test database created (e.g., `go_test_db`)
3. Database user with permissions to create/drop schemas

### Database Setup

```sql
-- Create test database
CREATE DATABASE go_test_db;

-- Grant permissions to test user
GRANT ALL PRIVILEGES ON DATABASE go_test_db TO your_user;
```

## Usage

### Step 1: Create Setup File (setup_test.go)

Create a setup file in your package that initializes your app components:

```go
package todo_test

import (
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/todo"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
)

// TodoApp holds the initialized todo application components
type TodoApp struct {
	*testsuite.AppContext
	Storage todo.Storage
	Service todo.Service
}

// setupTodoTest initializes the test suite
func setupTodoTest(t *testing.T) *testsuite.Suite {
	cfg := testsuite.InitTestConfig()

	suite, err := testsuite.NewSuite(cfg)
	if err != nil {
		t.Fatalf("Failed to create test suite: %v", err)
	}

	return suite
}

// initTodoApp initializes todo app components (like main function)
func initTodoApp(app *testsuite.AppContext) *TodoApp {
	storage := todo.NewStorage(app.Pool)
	service := todo.NewService(storage)

	return &TodoApp{
		AppContext: app,
		Storage:    storage,
		Service:    service,
	}
}
```

### Step 2: Create Test Helper (testing_helper_test.go)

Create domain-specific test helpers with DataXxx pattern:

```go
package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
)

// TestHelper provides utility methods for test fixtures
type TestHelper struct {
	pool *pgxpool.Pool
}

// NewTestHelper creates a new helper instance
func NewTestHelper(pool *pgxpool.Pool) *TestHelper {
	return &TestHelper{pool: pool}
}

// DataUser represents a user fixture for testing
type DataUser struct {
	Idx      int    // Index in the fixture array
	ID       int    // Actual database ID (populated after insert)
	Username string
	Email    string
	FullName string
	Password string // Plain text password for testing
}

// DataMember represents a member fixture for testing
type DataMember struct {
	Idx           int // Index in the fixture array
	ID            int // Actual database ID (populated after insert)
	UserID        int
	Name          string
	MonthlyIncome int
}

// GetAllUsers retrieves all users as a map[ID]User
func (h *TestHelper) GetAllUsers(ctx context.Context, t *testing.T) map[int]user.User {
	query := `SELECT id, username, email, full_name, created_at, updated_at FROM users ORDER BY id`
	rows, err := h.pool.Query(ctx, query)
	assert.Nil(t, err)
	defer rows.Close()

	users := make(map[int]user.User)
	for rows.Next() {
		var u user.User
		err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.FullName, &u.CreatedAt, &u.UpdatedAt)
		assert.Nil(t, err)
		users[u.ID] = u
	}
	assert.Nil(t, rows.Err())
	return users
}

// InsertUserWithHashedPassword inserts a user with hashed password
func (h *TestHelper) InsertUserWithHashedPassword(ctx context.Context, t *testing.T, username, email, fullName, password string) *user.User {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	assert.Nil(t, err)
	return h.InsertUser(ctx, t, username, email, fullName, string(hashedPassword))
}
```

### Step 3: Write Tests Following the Pattern

Your tests should follow this consistent structure:

```go
package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/stretchr/testify/assert"
)

func TestUserUpdateUsername(t *testing.T) {
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "User UpdateUsername", func() {

			// ========== 1. Declare Fixture Variables ==========
			var Users []DataUser

			// ========== 2. Define Test Structures ==========
			type input struct {
				userID      int
				newUsername string
			}
			type expected struct {
				success bool
				message string
			}
			type testRow struct {
				name     string
				input    *input
				expected *expected
			}

			// ========== 3. Setup Fixtures ==========
			suite.Setup(func(ctx context.Context, app *UserApp) {
				Users = []DataUser{
					{
						Idx:      0,
						Username: "testuser1",
						Email:    "test1@example.com",
						FullName: "Test User 1",
						Password: "password123",
					},
					{
						Idx:      1,
						Username: "testuser2",
						Email:    "test2@example.com",
						FullName: "Test User 2",
						Password: "password123",
					},
				}

				// Insert users and store actual database IDs
				for i, userData := range Users {
					insertedUser := app.Helper.InsertUserWithHashedPassword(ctx, t, userData.Username, userData.Email, userData.FullName, userData.Password)
					Users[i].ID = insertedUser.ID // Store actual database ID
				}
			})

			// ========== 4. Define Test Runner ==========
			runtest := func(t *testing.T, app *UserApp, r *testRow) {
				ctx := context.Background()

				// Get initial state BEFORE operation
				initialUsers := app.Helper.GetAllUsers(ctx, t)

				// Execute the service method being tested
				output := app.Service.UpdateUsername(ctx, &user.UpdateUsernameInput{
					TraceID:     "trace-test",
					ID:          r.input.userID,
					NewUsername: r.input.newUsername,
				})

				// Get state AFTER operation
				afterUsers := app.Helper.GetAllUsers(ctx, t)

				// Assert response matches expectations
				assert.Equal(t, r.expected.success, output.Success, r.name)
				assert.Equal(t, r.expected.message, output.Message, r.name)

				if r.expected.success == false {
					// Verify no users were modified on failure
					assert.Equal(t, initialUsers, afterUsers, r.name)
					return
				}

				// For successful update, verify specific fields
				assert.Equal(t, len(initialUsers), len(afterUsers), r.name)
				assert.Equal(t, r.input.newUsername, output.User.Username, r.name)

				// Verify the username was actually updated in database
				updatedUser, exists := afterUsers[r.input.userID]
				assert.True(t, exists, r.name+" - user should exist")
				assert.Equal(t, r.input.newUsername, updatedUser.Username, r.name)

				// Update initial state to reflect expected changes
				if initialUser, exists := initialUsers[r.input.userID]; exists {
					initialUser.Username = r.input.newUsername
					initialUser.UpdatedAt = updatedUser.UpdatedAt // UpdatedAt changes
					initialUsers[r.input.userID] = initialUser
				}

				// Verify ONLY the expected changes occurred
				assert.Equal(t, initialUsers, afterUsers, r.name+" - only username should change")
			}

			// ========== 5. Define Rows Runner ==========
			runRows := func(t *testing.T, app *UserApp, rows []*testRow) {
				for _, r := range rows {
					runtest(t, app, r)
				}
			}

			// ========== 6. Execute Test Scenarios ==========
			suite.Run(t, "UpdateUsername scenarios", func(t *testing.T, ctx context.Context, app *UserApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should update username successfully",
						input: &input{
							userID:      Users[0].ID,
							newUsername: "updateduser",
						},
						expected: &expected{
							success: true,
							message: "Username updated successfully",
						},
					},
					// ===== Validation Tests =====
					{
						name: "Should fail when username is empty",
						input: &input{
							userID:      Users[0].ID,
							newUsername: "",
						},
						expected: &expected{
							success: false,
							message: "New username is mandatory",
					},
				}
			}

			createFailedMemberMock := func() *user.GetMemberByIdOutput {
				return &user.GetMemberByIdOutput{
					Success: false,
					Message: "Member not found",
				}
			}

			suite.Run(t, "CreatePlan scenarios", func(t *testing.T, ctx context.Context, app *FinanceApp) {
				runRows(t, app, []*testRow{
					// ===== Success Tests =====
					{
						name: "Should create plan successfully with single income",
						input: &input{
							memberID: 12,
							month:    1,
							year:     2024,
							planItems: []finance.PlanItemInput{
								{
									Category:     finance.CPIncome,
									Title:        "Salary",
									SpendingPlan: 5000000,
								},
							},
						},
						expected: &expected{
							success:           true,
							message:           "Plan created successfully",
							expectedCountMock: 1,
						},
						userMock: createSuccessMemberMock(),
					},
				})
			})

		})
	})
}

```

## Key Concepts

### The "Main" Pattern

This suite works like a `main()` function for your tests:

1. **`setup_test.go`** - Your test "main" file that:
   - Defines `setupTodoTest()` - initializes the test suite
   - Defines `initTodoApp()` - initializes your app components (storage, service, etc.)
   - Holds your app context type (`TodoApp`)

2. **`*_test.go`** - Your actual tests that are **clean and focused**:
   - No database connection code
   - No pool/storage/service initialization
   - Just call `app.Service.Method()` and assert



## Mocking External Modules

When a service depends on another module (e.g. `user.Service` depends on
`permission.Service`), mock the external module instead of hitting its real
storage. Keep the module's own storage **real** (DB fixtures via `suite.Setup`),
and mock only the dependency with its generated counterfeiter fake (e.g.
`permission/fakes.ServiceFake`).

### Step 1: Wire the Mock in the App

Add the fake to your app struct and pass it to the service constructor in
`initXxxApp()`:

```go
type UserApp struct {
	*testsuite.AppContext
	Helper            *TestHelper
	Storage           user.Storage
	Service           user.Service
	PermissionSvcMock *permissionfakes.ServiceFake // mock of permission.Service
}

func initUserApp(app *testsuite.AppContext) *UserApp {
	helper := NewTestHelper(app.Pool)
	storage := user.NewStorage(app.Pool)
	permissionSvc := &permissionfakes.ServiceFake{} // mock, not the real service
	service := user.NewService(storage, permissionSvc)

	return &UserApp{
		AppContext:        app,
		Helper:            helper,
		Storage:           storage,
		Service:           service,
		PermissionSvcMock: permissionSvc,
	}
}
```

### Step 2: Carry the Mock Output in the Test Row

The `testRow` holds the mock's return value for the row, and `expected` tracks
how many times the mock must be called:

```go
type testRow struct {
	name            string
	input           *input
	expected        *expected
	permissionCheck *permission.CheckPermissionOutput // mock return for this row
}

type expected struct {
	success           bool
	message           string
	expectedCountMock int // how many times the mock must be called
}
```

### Step 3: Configure the Stub Per Row in runtest()

Before calling the service method, replace the stub. Inside the stub:

1. **Assert the input** the service passed to the mock
2. **Increment a `testsuite.Counter`**
3. **Return the row's mock output**

```go
runtest := func(t *testing.T, app *UserApp, r *testRow) {
	ctx := context.Background()
	counter := &testsuite.Counter{}

	app.PermissionSvcMock.CheckPermissionStub = func(ctx context.Context, input *permission.CheckPermissionInput) *permission.CheckPermissionOutput {
		assert.Equal(t, r.input.userID, input.UserID, r.name)
		counter.Inc()
		return r.permissionCheck
	}

	output := app.Service.UpdateUsername(ctx, &user.UpdateUsernameInput{
		TraceId:     "trace-test",
		Id:          r.input.userID,
		NewUsername: r.input.newUsername,
	})

	assert.Equal(t, r.expected.success, output.Success, r.name)
	assert.Equal(t, r.expected.message, output.Message, r.name)
	assert.Equal(t, r.expected.expectedCountMock, counter.Total(), r.name+" - mock call count")
}
```

### Step 4: Mock Factory Functions

Keep small factories that build the mock outputs so rows stay readable:

```go
createGrantedPermissionCheck := func() *permission.CheckPermissionOutput {
	return &permission.CheckPermissionOutput{
		Success:       true,
		Message:       "Permission check completed",
		HasPermission: true,
	}
}

createDeniedPermissionCheck := func() *permission.CheckPermissionOutput {
	return &permission.CheckPermissionOutput{
		Success:       true,
		Message:       "Permission check completed",
		HasPermission: false,
	}
}
```

### Mocking Rules

1. **Only the external module is mocked**: DB fixtures still come from
   `suite.Setup`, and state verification still compares DB before/after.
2. **`expectedCountMock` is mandatory**: it proves the mock was (or was not)
   called. Validation failures must call the mock `0` times; gated operations
   that reach the next step call it `1` time.
3. **Stub per row**: always set the stub inside `runtest`, never outside — each
   row controls its own mock.
4. **Assert mock inputs**: the stub asserts the args passed by the service so the
   contract is pinned (e.g. permission keys `"7:super_user"`,
   `"5:user:profile_update"`).



## API Reference

### Setup Functions (define once in setup_test.go)

#### `InitTestConfig() *config.Config`
Initializes test configuration from environment variables or defaults.

#### `NewSuite(cfg *config.Config) (*Suite, error)`
Creates a new test suite with database connection.

### Suite Methods

#### `suite.Describe(t *testing.T, description string, fn func())`
Groups related tests together.

#### `suite.Before(fn func())`
Registers a function to run before each test scenario for fixture setup.

#### `suite.Runs(t *testing.T, scenario string, fn func(t *testing.T, app *AppContext))`
Executes a test scenario with:
- Random schema creation
- Database migration execution
- AppContext initialization (Pool + Helper)
- Before hook execution
- Test execution
- Schema cleanup

**Key difference**: Your callback receives `*AppContext` to initialize your app components.

#### `suite.Close()`
Closes the database connection. Use with defer.

### AppContext

Passed to each `Runs()` callback:

```go
type AppContext struct {
    Pool   *pgxpool.Pool  // Database connection pool
    Helper *Helper        // Fixture helpers
}
```

Use this to initialize your app's storage/service/etc.

### Helper Methods

The `app.Helper` (or `suite.Helper`) provides fixture utilities:

#### `InsertTodo(ctx, title, description, isCompleted) (*Todo, error)`
Inserts a single todo and returns it.

#### `InsertTodos(ctx, fixtures) ([]*Todo, error)`
Inserts multiple todos from fixtures.

```go
fixtures := []testsuite.TodoFixture{
    {Title: "Todo 1", Description: "Desc 1", IsCompleted: false},
}
todos, err := app.Helper.InsertTodos(ctx, fixtures)
```

#### `GetTodoByID(ctx, id) (*Todo, error)`
Retrieves a todo by ID.

#### `CountTodos(ctx) (int, error)`
Returns the total number of todos.

#### `ClearTodos(ctx) error`
Removes all todos from the database.

#### `GetPool() *pgxpool.Pool`
Returns the connection pool (usually not needed - use `app.Pool` instead).

## How It Works

### Test Execution Flow

```
1. TestTodoCreate()
   ├─ setupTodoTest(t) → Suite with DB connection
   │
   └─ suite.Describe("Todo Create")
      ├─ suite.Before() → Register fixture setup
      │
      └─ suite.Runs("Scenario 1")
         ├─ Create random schema (e.g., test_abc123)
         ├─ Run migrations (create tables)
         ├─ Create AppContext{Pool, Helper}
         ├─ Run Before() hooks → Insert fixtures
         ├─ Call your test function with AppContext
         │  └─ initTodoApp(appCtx) → TodoApp{Storage, Service}
         │     └─ Your test logic: app.Service.Create(...)
         └─ Drop schema (cleanup)
```

### Schema Isolation

Each `Runs()` call:
1. Generates a random schema name (e.g., `test_abc123def456`)
2. Creates the schema: `CREATE SCHEMA test_abc123def456`
3. Runs migrations in that schema
4. Creates a connection pool with `search_path=test_abc123def456`
5. Executes your test
6. Drops the schema: `DROP SCHEMA test_abc123def456 CASCADE`

Tests cannot interfere with each other - complete isolation!

## Configuration

The suite uses  `InitTestConfig()` provides sensible defaults.

## Running Tests

```bash
# Run all tests
go test ./...

# Run specific test
go test ./src/app/todo -v

# Run with coverage
go test ./src/app/todo -cover
```

## Best Practices

1. **One setup_test.go per package**: Define `initXxxApp()` for your package
2. **Clean test files**: No boilerplate, just test logic
3. **Use descriptive names**: Make scenario and test row names clear
4. **Leverage isolation**: Each `Runs()` gets fresh data and schema
5. **Test real behavior**: Use actual services, not mocks
6. **Verify both output and DB state**: Check service output AND query database

## Example Test Output

```
=== RUN   TestTodoCreate
=== RUN   TestTodoCreate/Todo_Create
=== RUN   TestTodoCreate/Todo_Create/Scenario_1:_Create_valid_todos
=== RUN   TestTodoCreate/Todo_Create/Scenario_1:_Create_valid_todos/Create_simple_todo
=== RUN   TestTodoCreate/Todo_Create/Scenario_1:_Create_valid_todos/Create_todo_with_long_description
=== RUN   TestTodoCreate/Todo_Create/Scenario_2:_Create_todos_with_special_characters
=== RUN   TestTodoCreate/Todo_Create/Scenario_2:_Create_todos_with_special_characters/Create_todo_with_special_chars
--- PASS: TestTodoCreate (0.15s)
    --- PASS: TestTodoCreate/Todo_Create (0.15s)
        --- PASS: TestTodoCreate/Todo_Create/Scenario_1:_Create_valid_todos (0.08s)
            --- PASS: TestTodoCreate/Todo_Create/Scenario_1:_Create_valid_todos/Create_simple_todo (0.02s)
            --- PASS: TestTodoCreate/Todo_Create/Scenario_1:_Create_valid_todos/Create_todo_with_long_description (0.01s)
        --- PASS: TestTodoCreate/Todo_Create/Scenario_2:_Create_todos_with_special_characters (0.07s)
            --- PASS: TestTodoCreate/Todo_Create/Scenario_2:_Create_todos_with_special_characters/Create_todo_with_special_chars (0.01s)
PASS
```

## Troubleshooting

### Connection Issues
- Verify PostgreSQL is running
- Check database credentials
- Ensure test database exists
- Verify user has schema creation permissions

### Schema Cleanup Issues
- Check for open transactions blocking schema drop
- Verify no other connections are using the schema
- Check database logs for permission errors

### Migration Failures
- Verify SQL syntax in `suite.runMigrations()`
- Check for conflicting table names
- Ensure proper column types

## Testing Pattern Rules

### The 6-Step Pattern

Every test MUST follow this exact structure:

1. **Declare Fixture Variables** - Use `var DataXxx []DataXxx` pattern
2. **Define Test Structures** - input, expected, testRow types
3. **Setup Fixtures** - `suite.Setup()` with DataXxx initialization
4. **Define Test Runner** - `runtest()` with initial/after state comparison
5. **Define Rows Runner** - `runRows()` to iterate test rows
6. **Execute Test Scenarios** - `suite.Run()` with test rows array

### State Verification Pattern

**CRITICAL**: Every test MUST verify database state integrity:

```go
// 1. Get state BEFORE operation
initialData := app.Helper.GetAllXxx(ctx, t)

// 2. Execute service method
output := app.Service.MethodName(ctx, input)

// 3. Get state AFTER operation
afterData := app.Helper.GetAllXxx(ctx, t)

// 4. For FAILED operations - verify NO changes
if !output.Success {
    assert.Equal(t, initialData, afterData, r.name)
    return
}

// 5. For SUCCESS - verify ONLY expected changes
// Update initialData to reflect expected changes
initialData[id] = expectedValue

// 6. Assert initial equals after (proves ONLY expected changes)
assert.Equal(t, initialData, afterData, r.name)
```
