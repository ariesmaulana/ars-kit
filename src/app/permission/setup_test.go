package permission_test

import (
	"context"
	"testing"

	"github.com/ariesmaulana/ars-kit/database"
	"github.com/ariesmaulana/ars-kit/src/app/permission"
	testsuite "github.com/ariesmaulana/ars-kit/testing"
)

type PermissionApp struct {
	*testsuite.AppContext
	Helper  *TestHelper
	Storage permission.Storage
	Service permission.Service
}

type TestSuite struct {
	*testsuite.Suite
}

func (ts *TestSuite) Run(t *testing.T, scenario string, fn func(t *testing.T, ctx context.Context, app *PermissionApp)) {
	ts.Runs(t, scenario, func(t *testing.T, appCtx *testsuite.AppContext) {
		ctx := context.Background()
		app := initPermissionApp(appCtx)
		fn(t, ctx, app)
	})
}

func (ts *TestSuite) Setup(fn func(ctx context.Context, app *PermissionApp)) {
	ts.Before(func(appCtx *testsuite.AppContext) {
		ctx := context.Background()
		app := initPermissionApp(appCtx)
		fn(ctx, app)
	})
}

func initPermissionApp(app *testsuite.AppContext) *PermissionApp {
	helper := NewTestHelper(app.Pool)
	storage := permission.NewStorage(app.Pool)
	service := permission.NewService(storage)
	return &PermissionApp{
		AppContext: app,
		Helper:     helper,
		Storage:    storage,
		Service:    service,
	}
}

func RunTest(t *testing.T, testFunc func(t *testing.T, suite *TestSuite)) {
	t.Parallel()
	cfg := testsuite.InitTestConfig()

	baseSuite, err := testsuite.NewSuite(cfg, database.PermissionOnly)
	if err != nil {
		t.Fatalf("Failed to create test suite: %v", err)
	}

	t.Cleanup(func() {
		baseSuite.Close()
	})

	suite := &TestSuite{Suite: baseSuite}
	testFunc(t, suite)
}
