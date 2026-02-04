package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/lib/pq"
)

type Config struct {
	Customer  string
	Profile   string
	StartDate string
	EndDate   string
	Cluster   string
}

type DailyCount struct {
	Date    string
	Total   int
	Missing int
	Pct     float64
}

func main() {
	cfg := Config{}
	flag.StringVar(&cfg.Customer, "customer", "rentokil", "Customer ID")
	flag.StringVar(&cfg.Profile, "profile", "us-east-1", "Profile ID")
	flag.StringVar(&cfg.StartDate, "start", time.Now().AddDate(0, 0, -7).Format("2006-01-02"), "Start date (YYYY-MM-DD)")
	flag.StringVar(&cfg.EndDate, "end", time.Now().Format("2006-01-02"), "End date (YYYY-MM-DD)")
	flag.StringVar(&cfg.Cluster, "cluster", "us-east-1-prod", "Cluster name")
	flag.Parse()

	fmt.Println("==============================================")
	fmt.Println("Scorecard Sync Comparison Report")
	fmt.Println("==============================================")
	fmt.Printf("Customer: %s\n", cfg.Customer)
	fmt.Printf("Profile: %s\n", cfg.Profile)
	fmt.Printf("Date Range: %s to %s\n", cfg.StartDate, cfg.EndDate)
	fmt.Printf("Cluster: %s\n", cfg.Cluster)
	fmt.Println("==============================================")
	fmt.Println()

	// Connect to PostgreSQL
	pgConn, err := getPostgresConnection(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	defer pgConn.Close()

	// Connect to ClickHouse
	chConn, err := getClickHouseConnection()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to ClickHouse: %v\n", err)
		os.Exit(1)
	}
	defer chConn.Close()

	// Run comparisons
	if err := compareDirectorToHistoric(pgConn, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error comparing director to historic: %v\n", err)
	}

	if err := getClickHouseCounts(chConn, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error getting ClickHouse counts: %v\n", err)
	}

	if err := verifySampleScorecards(pgConn, chConn, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error verifying sample scorecards: %v\n", err)
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Report generated at: %s\n", time.Now().Format(time.RFC3339))
}

func getPostgresConnection(cfg Config) (*sql.DB, error) {
	dbName := fmt.Sprintf("%s-%s", cfg.Customer, cfg.Profile)

	// Get connection string using cresta-cli
	cmd := exec.Command("cresta-cli", "connstring", "-i", "--read-only", cfg.Cluster, cfg.Cluster, dbName)
	cmd.Env = append(os.Environ(), "AWS_REGION=us-east-1")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	connStr := strings.TrimSpace(string(output))
	return sql.Open("postgres", connStr)
}

func getClickHouseConnection() (clickhouse.Conn, error) {
	return clickhouse.Open(&clickhouse.Options{
		Addr: []string{"clickhouse-conversations.us-east-1-prod.internal.cresta.ai:9440"},
		Auth: clickhouse.Auth{
			Username: "admin",
			Password: "ItVIZdiPT8XQmD5Yox16ROpdNcjJYEEx",
		},
		TLS: &tls.Config{},
	})
}

func compareDirectorToHistoric(db *sql.DB, cfg Config) error {
	fmt.Println("=== Director vs Historic Schema (Missing Analysis) ===")
	fmt.Println()
	fmt.Println("Comparing submitted scorecards in director.scorecards to historic.scorecard_scores:")
	fmt.Println()

	query := `
SELECT
  TO_CHAR(d.created_at, 'YYYY-MM-DD') as date,
  COUNT(*) as total,
  COUNT(*) FILTER (WHERE h.scorecard_id IS NULL) as missing,
  ROUND(100.0 * COUNT(*) FILTER (WHERE h.scorecard_id IS NULL) / NULLIF(COUNT(*), 0), 1) as pct_missing
FROM director.scorecards d
LEFT JOIN (SELECT DISTINCT scorecard_id FROM historic.scorecard_scores
           WHERE customer_id = $1 AND profile_id = $2) h
  ON d.resource_id = h.scorecard_id
WHERE d.customer = $1 AND d.profile = $2
  AND d.submitted_at IS NOT NULL
  AND d.calibrated_scorecard_id IS NULL
  AND (d.scorecard_type IS NULL OR d.scorecard_type = 0)
  AND d.created_at >= $3::date
  AND d.created_at < $4::date + 1
GROUP BY TO_CHAR(d.created_at, 'YYYY-MM-DD')
ORDER BY date;
`

	rows, err := db.Query(query, cfg.Customer, cfg.Profile, cfg.StartDate, cfg.EndDate)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	fmt.Printf("%-12s | %8s | %8s | %8s\n", "Date", "Total", "Missing", "% Missing")
	fmt.Println("-------------|----------|----------|----------")

	var totalAll, missingAll int
	for rows.Next() {
		var c DailyCount
		if err := rows.Scan(&c.Date, &c.Total, &c.Missing, &c.Pct); err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}
		fmt.Printf("%-12s | %8d | %8d | %7.1f%%\n", c.Date, c.Total, c.Missing, c.Pct)
		totalAll += c.Total
		missingAll += c.Missing
	}

	if totalAll > 0 {
		fmt.Println("-------------|----------|----------|----------")
		pctAll := 100.0 * float64(missingAll) / float64(totalAll)
		fmt.Printf("%-12s | %8d | %8d | %7.1f%%\n", "TOTAL", totalAll, missingAll, pctAll)
	}

	fmt.Println()
	return nil
}

func getClickHouseCounts(conn clickhouse.Conn, cfg Config) error {
	fmt.Println("=== ClickHouse Scorecard Counts ===")
	fmt.Println()

	chDatabase := strings.ReplaceAll(cfg.Customer, "-", "_") + "_" + strings.ReplaceAll(cfg.Profile, "-", "_")

	query := fmt.Sprintf(`
SELECT
  toDate(scorecard_create_time) as date,
  count(*) as total
FROM %s.scorecard_d
WHERE scorecard_create_time >= toDate('%s')
  AND scorecard_create_time < toDate('%s') + 1
GROUP BY date
ORDER BY date
`, chDatabase, cfg.StartDate, cfg.EndDate)

	rows, err := conn.Query(context.Background(), query)
	if err != nil {
		return fmt.Errorf("clickhouse query failed: %w", err)
	}
	defer rows.Close()

	fmt.Printf("%-12s | %10s\n", "Date", "ClickHouse")
	fmt.Println("-------------|------------")

	for rows.Next() {
		var date time.Time
		var total uint64
		if err := rows.Scan(&date, &total); err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}
		fmt.Printf("%-12s | %10d\n", date.Format("2006-01-02"), total)
	}

	fmt.Println()
	return nil
}

func verifySampleScorecards(pgDB *sql.DB, chConn clickhouse.Conn, cfg Config) error {
	fmt.Println("=== Full ClickHouse Verification ===")
	fmt.Println()
	fmt.Println("Checking ALL scorecards from PostgreSQL director exist in ClickHouse...")
	fmt.Println()

	// Get ALL scorecard IDs from director
	query := `
SELECT resource_id
FROM director.scorecards
WHERE customer = $1 AND profile = $2
  AND submitted_at IS NOT NULL
  AND calibrated_scorecard_id IS NULL
  AND (scorecard_type IS NULL OR scorecard_type = 0)
  AND created_at >= $3::date
  AND created_at < $4::date + 1
ORDER BY resource_id;
`

	rows, err := pgDB.Query(query, cfg.Customer, cfg.Profile, cfg.StartDate, cfg.EndDate)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var allIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}
		allIDs = append(allIDs, id)
	}

	if len(allIDs) == 0 {
		fmt.Println("No scorecards found in the date range")
		return nil
	}

	fmt.Printf("Total scorecards in PostgreSQL director: %d\n", len(allIDs))

	// Check in batches of 500 to avoid query size limits
	chDatabase := strings.ReplaceAll(cfg.Customer, "-", "_") + "_" + strings.ReplaceAll(cfg.Profile, "-", "_")
	batchSize := 500
	var totalFound int
	var missingIDs []string

	for i := 0; i < len(allIDs); i += batchSize {
		end := i + batchSize
		if end > len(allIDs) {
			end = len(allIDs)
		}
		batch := allIDs[i:end]

		quotedIDs := make([]string, len(batch))
		for j, id := range batch {
			quotedIDs[j] = fmt.Sprintf("'%s'", id)
		}

		// Get IDs that exist in ClickHouse
		chQuery := fmt.Sprintf(`
SELECT DISTINCT scorecard_id
FROM %s.scorecard_d
WHERE scorecard_id IN (%s)
`, chDatabase, strings.Join(quotedIDs, ","))

		chRows, err := chConn.Query(context.Background(), chQuery)
		if err != nil {
			return fmt.Errorf("clickhouse query failed: %w", err)
		}

		foundSet := make(map[string]bool)
		for chRows.Next() {
			var id string
			if err := chRows.Scan(&id); err != nil {
				chRows.Close()
				return fmt.Errorf("scan failed: %w", err)
			}
			foundSet[id] = true
		}
		chRows.Close()

		totalFound += len(foundSet)

		// Find missing IDs in this batch
		for _, id := range batch {
			if !foundSet[id] {
				missingIDs = append(missingIDs, id)
			}
		}

		fmt.Printf("  Checked %d/%d...\r", end, len(allIDs))
	}

	fmt.Printf("\n")
	fmt.Printf("Found in ClickHouse: %d\n", totalFound)
	fmt.Printf("Missing from ClickHouse: %d\n", len(missingIDs))

	if len(missingIDs) == 0 {
		fmt.Println("✓ All scorecards found in ClickHouse")
	} else {
		pct := 100.0 * float64(len(missingIDs)) / float64(len(allIDs))
		fmt.Printf("✗ %.1f%% scorecards missing from ClickHouse\n", pct)

		// List first 20 missing IDs
		fmt.Println()
		fmt.Println("Missing scorecard IDs (first 20):")
		limit := 20
		if len(missingIDs) < limit {
			limit = len(missingIDs)
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("  - %s\n", missingIDs[i])
		}
		if len(missingIDs) > 20 {
			fmt.Printf("  ... and %d more\n", len(missingIDs)-20)
		}
	}

	fmt.Println()
	return nil
}
