package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
)

// UserApp holds the initialized user application components
type UserApp struct {
	*testsuite.AppContext
	Helper  *TestHelper
	Storage user.Storage
	Service user.Service
}

// TestSuite wraps testsuite.Suite for user tests
type TestSuite struct {
	*testsuite.Suite
}

// Run executes a test scenario with initialized UserApp and context
func (ts *TestSuite) Run(t *testing.T, scenario string, fn func(t *testing.T, ctx context.Context, app *UserApp)) {
	ts.Runs(t, scenario, func(t *testing.T, appCtx *testsuite.AppContext) {
		ctx := context.Background()
		app := initUserApp(appCtx)
		fn(t, ctx, app)
	})
}

// Setup registers a function to run before each test scenario with initialized UserApp and context
func (ts *TestSuite) Setup(fn func(ctx context.Context, app *UserApp)) {
	ts.Before(func(appCtx *testsuite.AppContext) {
		ctx := context.Background()
		app := initUserApp(appCtx)
		fn(ctx, app)
	})
}

// initUserApp initializes user app components from the app context
func initUserApp(app *testsuite.AppContext) *UserApp {
	helper := NewTestHelper(app.Pool)
	storage := user.NewStorage(app.Pool)
	service := user.NewService(storage)

	return &UserApp{
		AppContext: app,
		Helper:     helper,
		Storage:    storage,
		Service:    service,
	}
}

// RunTest is a wrapper that automatically sets up and tears down the test suite
func RunTest(t *testing.T, testFunc func(t *testing.T, suite *TestSuite)) {
	t.Parallel()
	cfg := testsuite.InitTestConfig()

	baseSuite, err := testsuite.NewSuite(cfg, "users.sql")
	if err != nil {
		t.Fatalf("Failed to create test suite: %v", err)
	}

	t.Cleanup(func() {
		baseSuite.Close()
	})

	suite := &TestSuite{Suite: baseSuite}
	testFunc(t, suite)
}
