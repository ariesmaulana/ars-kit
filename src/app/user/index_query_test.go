package user_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ariesmaulana/ars-kit/src/app/user"
	"github.com/jackc/pgx/v5"
)

// TestIndexQuery is a performance test that populates random N data
// and runs EXPLAIN ANALYZE to help identify optimal indexes.
//
// Run with: go test -run TestIndexQuery -v ./src/app/user
//
// Environment variables:
//
//	INDEX_TEST_ROWS=10000   - Number of members to generate (default: 10000)
//	INDEX_TEST_USERS=10     - Number of users to create (default: 10)
//
// The test will:
// 1. Create INDEX_TEST_USERS users
// 2. Generate INDEX_TEST_ROWS random members distributed across users
// 3. Run EXPLAIN (ANALYZE, BUFFERS) on GetMembersByUserId query
// 4. Print the query plan to help identify index effectiveness
//
// No assertions are made - this is purely for analysis.
func TestIndexQuery(t *testing.T) {
	t.Skip()
	RunTest(t, func(t *testing.T, suite *TestSuite) {
		suite.Describe(t, "Index Query Analysis", func() {

			var targetUserId int
			var totalRows int
			var totalUsers int
			var seed int64

			suite.Setup(func(ctx context.Context, app *UserApp) {
				// Get configuration from environment variables
				totalRows = getEnvInt("INDEX_TEST_ROWS", 10000)
				totalUsers = getEnvInt("INDEX_TEST_USERS", 10)
				seed = time.Now().UnixNano()

				t.Logf("=== Index Query Test Configuration ===")
				t.Logf("Seed: %d", seed)
				t.Logf("Total users to create: %d", totalUsers)
				t.Logf("Total members to create: %d", totalRows)
				t.Logf("=====================================")

				// Seed random number generator
				rng := rand.New(rand.NewSource(seed))

				// Step 1: Create users
				t.Logf("Creating %d users...", totalUsers)
				userIds := make([]int, 0, totalUsers)
				for i := 0; i < totalUsers; i++ {
					username := fmt.Sprintf("user_%d_%d", seed, i)
					email := fmt.Sprintf("user_%d_%d@example.com", seed, i)
					fullName := fmt.Sprintf("Test User %d", i)
					password := "hashedpassword123"

					insertedUser := app.Helper.InsertUser(ctx, t, username, email, fullName, password)
					userIds = append(userIds, insertedUser.Id)
				}

				// Pick a target user (middle one) to query later
				targetUserId = userIds[totalUsers/2]
				t.Logf("Target user ID for query: %d", targetUserId)

				// Step 2: Bulk insert members using COPY FROM for performance
				t.Logf("Bulk inserting %d members...", totalRows)
				start := time.Now()

				// Generate random member data
				memberRows := make([][]interface{}, 0, totalRows)
				for i := 0; i < totalRows; i++ {
					userId := userIds[rng.Intn(totalUsers)]
					name := fmt.Sprintf("Member_%d", i)
					monthlyIncome := rng.Intn(10000000)

					memberRows = append(memberRows, []interface{}{
						userId,
						name,
						monthlyIncome,
						time.Now(),
						time.Now(),
					})
				}

				// Use CopyFrom for fast bulk insert
				copyCount, err := app.Pool.CopyFrom(
					ctx,
					pgx.Identifier{"members"},
					[]string{"user_id", "name", "monthly_income", "created_at", "updated_at"},
					pgx.CopyFromRows(memberRows),
				)

				if err != nil {
					t.Fatalf("Failed to bulk insert members: %v", err)
				}

				elapsed := time.Since(start)
				t.Logf("Inserted %d members in %v (%.0f rows/sec)",
					copyCount, elapsed, float64(copyCount)/elapsed.Seconds())

				// Get count of members for target user
				var targetUserMemberCount int
				err = app.Pool.QueryRow(ctx,
					"SELECT COUNT(*) FROM members WHERE user_id = $1",
					targetUserId,
				).Scan(&targetUserMemberCount)

				if err != nil {
					t.Fatalf("Failed to count members for target user: %v", err)
				}

				t.Logf("Target user %d has %d members", targetUserId, targetUserMemberCount)
			})

			suite.Run(t, "Query plan analysis for GetMembersByUserId", func(t *testing.T, ctx context.Context, app *UserApp) {
				t.Logf("\n=== Running EXPLAIN ANALYZE on GetMembersByUserId query ===")
				t.Logf("Query: SELECT id, user_id, name, monthly_income, created_at, updated_at FROM members WHERE user_id = $1")
				t.Logf("Parameter: user_id = %d", targetUserId)
				t.Logf("")

				// Run EXPLAIN ANALYZE on the actual query used by storage layer
				explainQuery := `
					EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
					SELECT id, user_id, name, monthly_income, created_at, updated_at
					FROM members
					WHERE user_id = $1
				`

				start := time.Now()
				rows, err := app.Pool.Query(ctx, explainQuery, targetUserId)
				if err != nil {
					t.Fatalf("Failed to run EXPLAIN ANALYZE: %v", err)
				}
				defer rows.Close()

				t.Logf("=== Query Plan Output ===")
				lineNum := 1
				for rows.Next() {
					var planLine string
					if err := rows.Scan(&planLine); err != nil {
						t.Fatalf("Failed to scan explain output: %v", err)
					}
					t.Logf("%3d: %s", lineNum, planLine)
					lineNum++
				}

				if err := rows.Err(); err != nil {
					t.Fatalf("Error iterating explain output: %v", err)
				}

				elapsed := time.Since(start)
				t.Logf("")
				t.Logf("=== Analysis Complete ===")
				t.Logf("EXPLAIN ANALYZE execution time: %v", elapsed)
				t.Logf("")
				t.Logf("How to interpret the output:")
				t.Logf("- Look for 'Index Scan' vs 'Seq Scan' - Index Scan is faster")
				t.Logf("- Check 'actual time' - shows actual query execution time")
				t.Logf("- Review 'rows' - shows how many rows were scanned vs returned")
				t.Logf("- Examine 'Buffers' - shows disk I/O (lower is better)")
				t.Logf("")
				t.Logf("If you see 'Seq Scan', the index might not be used. Reasons:")
				t.Logf("1. Not enough data to make index worthwhile (increase INDEX_TEST_ROWS)")
				t.Logf("2. Index doesn't exist (check SQL schema)")
				t.Logf("3. Query planner chose seq scan (try VACUUM ANALYZE)")
				t.Logf("")
				t.Logf("Re-run with different parameters:")
				t.Logf("  INDEX_TEST_ROWS=50000 INDEX_TEST_USERS=20 go test -run TestIndexQuery -v ./src/app/user")

				// Also run a simple timing test without EXPLAIN
				t.Logf("\n=== Actual Query Performance (without EXPLAIN overhead) ===")
				timingStart := time.Now()

				var members []user.Member
				timingRows, err := app.Pool.Query(ctx,
					`SELECT id, user_id, name, monthly_income, created_at, updated_at
					 FROM members WHERE user_id = $1`,
					targetUserId,
				)
				if err != nil {
					t.Fatalf("Failed to run timing query: %v", err)
				}
				defer timingRows.Close()

				for timingRows.Next() {
					var m user.Member
					err := timingRows.Scan(&m.Id, &m.UserId, &m.Name, &m.MonthlyIncome, &m.CreatedAt, &m.UpdatedAt)
					if err != nil {
						t.Fatalf("Failed to scan member: %v", err)
					}
					members = append(members, m)
				}

				if err := timingRows.Err(); err != nil {
					t.Fatalf("Error iterating members: %v", err)
				}

				timingElapsed := time.Since(timingStart)
				t.Logf("Fetched %d members in %v (%.2f ms)", len(members), timingElapsed, float64(timingElapsed.Microseconds())/1000.0)
				t.Logf("Average time per row: %.2f μs", float64(timingElapsed.Microseconds())/float64(len(members)))
				t.Logf("==============================================")
			})

		})
	})
}

// getEnvInt reads an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultValue
	}
	return val
}
