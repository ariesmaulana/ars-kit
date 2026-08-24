package user_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/database"
	permissionfakes "github.com/ariesmaulana/ars-kit/src/app/permission/fakes"
	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/ariesmaulana/ars-kit/src/clock"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
)

// UserApp holds the initialized user application components
type UserApp struct {
	*testsuite.AppContext
	Helper            *TestHelper
	Storage           user.Storage
	Service           user.Service
	PermissionSvcMock *permissionfakes.ServiceFake
}

// TestSuite wraps testsuite.Suite for user tests.
// Clock optionally pins "now" for services built by Setup/Run
// (nil = real time). Set it before Setup/Run, e.g. suite.Clock = clock.Fixed(t0).
type TestSuite struct {
	*testsuite.Suite
	Clock clock.Source
}

// Run executes a test scenario with initialized UserApp and context
func (ts *TestSuite) Run(t *testing.T, scenario string, fn func(t *testing.T, ctx context.Context, app *UserApp)) {
	ts.Runs(t, scenario, func(t *testing.T, appCtx *testsuite.AppContext) {
		ctx := context.Background()
		app := initUserAppWithThrottle(appCtx, user.DefaultLoginThrottleConfig(), ts.Clock)
		fn(t, ctx, app)
	})
}

// Setup registers a function to run before each test scenario with initialized UserApp and context
func (ts *TestSuite) Setup(fn func(ctx context.Context, app *UserApp)) {
	ts.Before(func(appCtx *testsuite.AppContext) {
		ctx := context.Background()
		app := initUserAppWithThrottle(appCtx, user.DefaultLoginThrottleConfig(), ts.Clock)
		fn(ctx, app)
	})
}

// initUserApp initializes user app components from the app context with the
// default login-throttle policy. The permission module is mocked
// (permissionfakes.ServiceFake) so tests can control permission outcomes per
// test row without a database.
func initUserApp(app *testsuite.AppContext) *UserApp {
	return initUserAppWithThrottle(app, user.DefaultLoginThrottleConfig())
}

// initUserAppWithThrottle is initUserApp with a caller-provided login-throttle
// policy so throttle-specific tests can use small, fast windows instead of the
// 15-minute defaults. An optional clock.Source pins "now" for the service
// (per-instance, no global state) so time-dependent assertions are deterministic.
func initUserAppWithThrottle(app *testsuite.AppContext, throttle user.LoginThrottleConfig, clockSource ...clock.Source) *UserApp {
	helper := NewTestHelper(app.Pool)
	storage := user.NewStorage(app.Pool)
	permissionService := &permissionfakes.ServiceFake{}
	jwtService := user.NewJWTService(user.JWTConfig{
		SecretKey:       "test-secret",
		ExpirationHours: 24,
	})
	service := user.NewService(storage, permissionService, throttle, jwtService, clockSource...)

	return &UserApp{
		AppContext:        app,
		Helper:            helper,
		Storage:           storage,
		Service:           service,
		PermissionSvcMock: permissionService,
	}
}

// RunTest is a wrapper that automatically sets up and tears down the test suite
func RunTest(t *testing.T, testFunc func(t *testing.T, suite *TestSuite)) {
	t.Parallel()
	cfg := testsuite.InitTestConfig()

	baseSuite, err := testsuite.NewSuite(cfg, database.UserOnly)
	if err != nil {
		t.Fatalf("Failed to create test suite: %v", err)
	}

	t.Cleanup(func() {
		baseSuite.Close()
	})

	suite := &TestSuite{Suite: baseSuite}
	testFunc(t, suite)
}
