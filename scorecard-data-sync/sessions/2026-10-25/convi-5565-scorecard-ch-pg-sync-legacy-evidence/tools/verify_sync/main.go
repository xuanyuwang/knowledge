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
	"github.com/cresta/go-servers/shared/scoring"
	_ "github.com/lib/pq"
)

// CriterionNameMap maps criterion_id to display_name
type CriterionNameMap map[string]string

// PostgreSQL scorecard data
type PGScorecard struct {
	ResourceID  string
	Customer    string
	Profile     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	SubmittedAt sql.NullTime
	Score       sql.NullFloat64
	TemplateID  string
	TemplateRev string
	AgentUserID string
	CreatorID   sql.NullString
	SubmitterID sql.NullString
}

// PostgreSQL score data
type PGScore struct {
	ResourceID          string
	ScorecardID         string
	CriterionIdentifier string
	NumericValue        sql.NullFloat64
	AIValue             sql.NullFloat64
	TextValue           sql.NullString
	NotApplicable       bool
	AIScored            bool
}

// ClickHouse scorecard data
type CHScorecard struct {
	ScorecardID         string
	CustomerID          string
	ProfileID           string
	ScorecardCreateTime time.Time
	ScorecardLastUpdate time.Time
	ScorecardSubmitTime time.Time
	Score               float64
	TemplateID          string
	TemplateRev         string
	AgentUserID         string
	CreatorUserID       string
	SubmitterUserID     string
	UpdateTime          time.Time
}

// ClickHouse score data
type CHScore struct {
	ScoreID             string
	ScorecardID         string
	CriterionID         string
	NumericValue        float64
	AIValue             float64
	TextValue           string
	NotApplicable       bool
	AIScored            bool
	ScorecardSubmitTime time.Time
	UpdateTime          time.Time
}

func main() {
	// Parse flags
	scorecardName := flag.String("name", "", "Full scorecard name (e.g., customers/cox/profiles/sales/scorecards/019bd22d-fda5-745f-92df-9cb8f1644950)")
	pgConnStr := flag.String("pg", "", "PostgreSQL connection string (if empty, uses cresta-cli)")
	chConnStr := flag.String("ch", "", "ClickHouse connection string (unused, hardcoded for now)")
	_ = chConnStr // unused
	flag.Parse()

	if *scorecardName == "" {
		fmt.Println("Usage: ./verify_sync -name <scorecard-full-name>")
		fmt.Println("\nExample:")
		fmt.Println("  ./verify_sync -name \"customers/cox/profiles/sales/scorecards/019bd22d-fda5-745f-92df-9cb8f1644950\"")
		fmt.Println("\nOptional flags:")
		fmt.Println("  -pg         PostgreSQL connection string (default: from cresta-cli)")
		os.Exit(1)
	}

	// Parse scorecard name: customers/{customer}/profiles/{profile}/scorecards/{id}
	customer, profile, scorecardID, err := parseScorecardName(*scorecardName)
	if err != nil {
		fmt.Printf("Error parsing scorecard name: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Parsed: customer=%s, profile=%s, scorecard=%s\n", customer, profile, scorecardID)

	ctx := context.Background()

	// Get PostgreSQL connection string
	pgConn := *pgConnStr
	if pgConn == "" {
		fmt.Println("Getting PostgreSQL connection string from cresta-cli...")
		cmd := exec.Command("cresta-cli", "connstring", "-i", "--read-only", "chat-staging", "chat-staging", "cox-sales")
		output, err := cmd.Output()
		if err != nil {
			fmt.Printf("Error getting connection string: %v\n", err)
			fmt.Println("Please provide -pg flag manually")
			os.Exit(1)
		}
		pgConn = strings.TrimSpace(string(output))
		fmt.Printf("Got connection string: %s\n", maskPassword(pgConn))
	}

	// Connect to PostgreSQL
	fmt.Println("\n--- Connecting to PostgreSQL ---")
	pgDB, err := sql.Open("postgres", pgConn)
	if err != nil {
		fmt.Printf("Error connecting to PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	defer pgDB.Close()

	if err := pgDB.Ping(); err != nil {
		fmt.Printf("Error pinging PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("PostgreSQL connected successfully")

	// Connect to ClickHouse
	// Database name is {customer}_{profile} with hyphens replaced by underscores
	chDatabase := strings.ReplaceAll(customer, "-", "_") + "_" + strings.ReplaceAll(profile, "-", "_")
	fmt.Printf("\n--- Connecting to ClickHouse (database: %s) ---\n", chDatabase)
	chConn, err := connectClickHouse(chDatabase)
	if err != nil {
		fmt.Printf("Error connecting to ClickHouse: %v\n", err)
		os.Exit(1)
	}
	defer chConn.Close()
	fmt.Println("ClickHouse connected successfully")

	// Query PostgreSQL
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("POSTGRESQL DATA")
	fmt.Println(strings.Repeat("=", 60))
	pgScorecard, pgScores, err := queryPostgres(ctx, pgDB, customer, profile, scorecardID)
	if err != nil {
		fmt.Printf("Error querying PostgreSQL: %v\n", err)
		os.Exit(1)
	}

	var criterionNames CriterionNameMap
	if pgScorecard == nil {
		fmt.Println("Scorecard NOT FOUND in PostgreSQL")
	} else {
		printPGScorecard(pgScorecard)

		// Fetch criterion display names from template using shared/scoring parser
		criterionNames, err = queryCriterionNames(ctx, pgDB, customer, profile, pgScorecard.TemplateID, pgScorecard.TemplateRev)
		if err != nil {
			fmt.Printf("Warning: could not fetch criterion names: %v\n", err)
			criterionNames = make(CriterionNameMap)
		}

		printPGScores(pgScores, criterionNames)
	}

	// Query ClickHouse scorecard table (RAW - all versions)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CLICKHOUSE DATA (scorecard table - RAW, all versions)")
	fmt.Println(strings.Repeat("=", 60))
	chScorecardRaw, err := queryClickHouseScorecardRaw(ctx, chConn, customer, profile, scorecardID)
	if err != nil {
		fmt.Printf("Error querying ClickHouse scorecard (raw): %v\n", err)
	} else if len(chScorecardRaw) == 0 {
		fmt.Println("No raw scorecard rows found")
	} else {
		fmt.Printf("\nFound %d row version(s):\n", len(chScorecardRaw))
		for i, sc := range chScorecardRaw {
			fmt.Printf("  Version %d:\n", i+1)
			fmt.Printf("    submit_time=%s, update_time=%s, score=%.2f\n",
				formatCHTime(sc.ScorecardSubmitTime), sc.UpdateTime.Format(time.RFC3339), sc.Score)
		}
	}

	// Query ClickHouse scorecard table (FINAL - merged)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CLICKHOUSE DATA (scorecard table - FINAL)")
	fmt.Println(strings.Repeat("=", 60))
	chScorecard, err := queryClickHouseScorecard(ctx, chConn, customer, profile, scorecardID)
	if err != nil {
		fmt.Printf("Error querying ClickHouse scorecard: %v\n", err)
		os.Exit(1)
	}

	if chScorecard == nil {
		fmt.Println("Scorecard NOT FOUND in ClickHouse")
	} else {
		printCHScorecard(chScorecard)
	}

	// Query ClickHouse score table (RAW - all versions)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CLICKHOUSE DATA (score table - RAW, all versions)")
	fmt.Println(strings.Repeat("=", 60))
	chScoresRaw, err := queryClickHouseScoresRaw(ctx, chConn, customer, profile, scorecardID)
	if err != nil {
		fmt.Printf("Error querying ClickHouse scores (raw): %v\n", err)
	} else if len(chScoresRaw) == 0 {
		fmt.Println("No raw score rows found")
	} else {
		fmt.Printf("\nFound %d raw score row(s):\n", len(chScoresRaw))
		for i, s := range chScoresRaw {
			fmt.Printf("  Row %d: criterion=%s, submit_time=%s, update_time=%s\n",
				i+1, s.CriterionID, formatCHTime(s.ScorecardSubmitTime), s.UpdateTime.Format(time.RFC3339))
		}
	}

	// Query ClickHouse score table (FINAL)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("CLICKHOUSE DATA (score table - FINAL)")
	fmt.Println(strings.Repeat("=", 60))
	chScores, err := queryClickHouseScores(ctx, chConn, customer, profile, scorecardID)
	if err != nil {
		fmt.Printf("Error querying ClickHouse scores: %v\n", err)
		os.Exit(1)
	}

	if len(chScores) == 0 {
		fmt.Println("No scores found in ClickHouse")
	} else {
		printCHScores(chScores, criterionNames)
	}

	// Compare and validate
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("VALIDATION RESULTS")
	fmt.Println(strings.Repeat("=", 60))
	validateSync(pgScorecard, pgScores, chScorecard, chScores, criterionNames)
}

func connectClickHouse(database string) (clickhouse.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"clickhouse-conversations.chat-staging.internal.cresta.ai:9440"},
		Auth: clickhouse.Auth{
			Database: database,
			Username: os.Getenv("CH_USER"),
			Password: os.Getenv("CH_PASS"),
		},
		TLS: &tls.Config{
			InsecureSkipVerify: true,
		},
		Debug: false,
	})
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, err
	}

	return conn, nil
}

// queryCriterionNames fetches the template JSON and extracts criterion display names
// using the shared/scoring template parser
func queryCriterionNames(ctx context.Context, db *sql.DB, customer, profile, templateID, templateRev string) (CriterionNameMap, error) {
	var templateJSON string
	err := db.QueryRowContext(ctx, `
		SELECT template
		FROM director.scorecard_template_revisions
		WHERE customer = $1 AND profile = $2 AND template_id = $3 AND resource_id = $4
	`, customer, profile, templateID, templateRev).Scan(&templateJSON)
	if err != nil {
		return nil, fmt.Errorf("query template: %w", err)
	}

	// Use shared/scoring parser to parse the template
	templateStructure, err := scoring.ParseScorecardTemplateStructure(templateJSON)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	// Extract criterion names using GetChaptersAndCriteria
	_, criteria := templateStructure.GetChaptersAndCriteria()

	nameMap := make(CriterionNameMap)
	for id, criterion := range criteria {
		nameMap[id] = criterion.GetDisplayName()
	}

	return nameMap, nil
}

func queryPostgres(ctx context.Context, db *sql.DB, customer, profile, scorecardID string) (*PGScorecard, []PGScore, error) {
	// Query scorecard
	var sc PGScorecard
	err := db.QueryRowContext(ctx, `
		SELECT resource_id, customer, profile, created_at, updated_at, submitted_at,
		       score, template_id, template_revision, agent_user_id, creator_user_id, submitter_user_id
		FROM director.scorecards
		WHERE customer = $1 AND profile = $2 AND resource_id = $3
	`, customer, profile, scorecardID).Scan(
		&sc.ResourceID, &sc.Customer, &sc.Profile,
		&sc.CreatedAt, &sc.UpdatedAt, &sc.SubmittedAt,
		&sc.Score, &sc.TemplateID, &sc.TemplateRev,
		&sc.AgentUserID, &sc.CreatorID, &sc.SubmitterID,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query scorecard: %w", err)
	}

	// Query scores
	rows, err := db.QueryContext(ctx, `
		SELECT resource_id, scorecard_id, criterion_identifier, numeric_value, ai_value, text_value, not_applicable, ai_scored
		FROM director.scores
		WHERE scorecard_id = $1
		ORDER BY criterion_identifier
	`, scorecardID)
	if err != nil {
		return nil, nil, fmt.Errorf("query scores: %w", err)
	}
	defer rows.Close()

	var scores []PGScore
	for rows.Next() {
		var s PGScore
		if err := rows.Scan(&s.ResourceID, &s.ScorecardID, &s.CriterionIdentifier,
			&s.NumericValue, &s.AIValue, &s.TextValue, &s.NotApplicable, &s.AIScored); err != nil {
			return nil, nil, fmt.Errorf("scan score: %w", err)
		}
		scores = append(scores, s)
	}

	return &sc, scores, nil
}

// queryClickHouseScorecardRaw returns all versions of a scorecard row (without FINAL)
func queryClickHouseScorecardRaw(ctx context.Context, conn clickhouse.Conn, customer, profile, scorecardID string) ([]CHScorecard, error) {
	rows, err := conn.Query(ctx, `
		SELECT
			scorecard_id,
			customer_id,
			profile_id,
			scorecard_create_time,
			scorecard_last_update_time,
			scorecard_submit_time,
			score,
			scorecard_template_id,
			scorecard_template_revision,
			agent_user_id,
			creator_user_id,
			submitter_user_id,
			update_time
		FROM scorecard_d
		WHERE customer_id = $1 AND profile_id = $2 AND scorecard_id = $3
		ORDER BY update_time
	`, customer, profile, scorecardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scorecards []CHScorecard
	for rows.Next() {
		var sc CHScorecard
		if err := rows.Scan(
			&sc.ScorecardID, &sc.CustomerID, &sc.ProfileID,
			&sc.ScorecardCreateTime, &sc.ScorecardLastUpdate, &sc.ScorecardSubmitTime,
			&sc.Score, &sc.TemplateID, &sc.TemplateRev,
			&sc.AgentUserID, &sc.CreatorUserID, &sc.SubmitterUserID,
			&sc.UpdateTime,
		); err != nil {
			return nil, err
		}
		scorecards = append(scorecards, sc)
	}
	return scorecards, nil
}

func queryClickHouseScorecard(ctx context.Context, conn clickhouse.Conn, customer, profile, scorecardID string) (*CHScorecard, error) {
	row := conn.QueryRow(ctx, `
		SELECT
			scorecard_id,
			customer_id,
			profile_id,
			scorecard_create_time,
			scorecard_last_update_time,
			scorecard_submit_time,
			score,
			scorecard_template_id,
			scorecard_template_revision,
			agent_user_id,
			creator_user_id,
			submitter_user_id,
			update_time
		FROM scorecard_d FINAL
		WHERE customer_id = $1 AND profile_id = $2 AND scorecard_id = $3
	`, customer, profile, scorecardID)

	var sc CHScorecard
	err := row.Scan(
		&sc.ScorecardID, &sc.CustomerID, &sc.ProfileID,
		&sc.ScorecardCreateTime, &sc.ScorecardLastUpdate, &sc.ScorecardSubmitTime,
		&sc.Score, &sc.TemplateID, &sc.TemplateRev,
		&sc.AgentUserID, &sc.CreatorUserID, &sc.SubmitterUserID,
		&sc.UpdateTime,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &sc, nil
}

// queryClickHouseScoresRaw returns all versions of score rows (without FINAL)
func queryClickHouseScoresRaw(ctx context.Context, conn clickhouse.Conn, customer, profile, scorecardID string) ([]CHScore, error) {
	rows, err := conn.Query(ctx, `
		SELECT
			score_id,
			scorecard_id,
			criterion_id,
			numeric_value,
			ai_value,
			text_value,
			not_applicable,
			ai_scored,
			scorecard_submit_time,
			update_time
		FROM score_d
		WHERE customer_id = $1 AND profile_id = $2 AND scorecard_id = $3
		ORDER BY criterion_id, update_time
	`, customer, profile, scorecardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []CHScore
	for rows.Next() {
		var s CHScore
		if err := rows.Scan(&s.ScoreID, &s.ScorecardID, &s.CriterionID,
			&s.NumericValue, &s.AIValue, &s.TextValue, &s.NotApplicable, &s.AIScored,
			&s.ScorecardSubmitTime, &s.UpdateTime); err != nil {
			return nil, err
		}
		scores = append(scores, s)
	}
	return scores, nil
}

func queryClickHouseScores(ctx context.Context, conn clickhouse.Conn, customer, profile, scorecardID string) ([]CHScore, error) {
	rows, err := conn.Query(ctx, `
		SELECT
			score_id,
			scorecard_id,
			criterion_id,
			numeric_value,
			ai_value,
			text_value,
			not_applicable,
			ai_scored,
			scorecard_submit_time,
			update_time
		FROM score_d FINAL
		WHERE customer_id = $1 AND profile_id = $2 AND scorecard_id = $3
		ORDER BY criterion_id
	`, customer, profile, scorecardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []CHScore
	for rows.Next() {
		var s CHScore
		if err := rows.Scan(&s.ScoreID, &s.ScorecardID, &s.CriterionID,
			&s.NumericValue, &s.AIValue, &s.TextValue, &s.NotApplicable, &s.AIScored,
			&s.ScorecardSubmitTime, &s.UpdateTime); err != nil {
			return nil, err
		}
		scores = append(scores, s)
	}

	return scores, nil
}

func printPGScorecard(sc *PGScorecard) {
	fmt.Printf("\nScorecard:\n")
	fmt.Printf("  Resource ID:   %s\n", sc.ResourceID)
	fmt.Printf("  Customer:      %s\n", sc.Customer)
	fmt.Printf("  Profile:       %s\n", sc.Profile)
	fmt.Printf("  Template:      %s (rev %s)\n", sc.TemplateID, sc.TemplateRev)
	fmt.Printf("  Agent User ID: %s\n", sc.AgentUserID)
	if sc.CreatorID.Valid {
		fmt.Printf("  Creator ID:    %s\n", sc.CreatorID.String)
	}
	fmt.Printf("  Created At:    %s\n", sc.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Updated At:    %s\n", sc.UpdatedAt.Format(time.RFC3339))
	if sc.SubmittedAt.Valid {
		fmt.Printf("  Submitted At:  %s\n", sc.SubmittedAt.Time.Format(time.RFC3339))
		if sc.SubmitterID.Valid {
			fmt.Printf("  Submitter ID:  %s\n", sc.SubmitterID.String)
		}
	} else {
		fmt.Printf("  Submitted At:  NULL (not submitted)\n")
	}
	if sc.Score.Valid {
		fmt.Printf("  Score:         %.2f\n", sc.Score.Float64)
	} else {
		fmt.Printf("  Score:         NULL\n")
	}
}

func printPGScores(scores []PGScore, criterionNames CriterionNameMap) {
	fmt.Printf("\nScores (%d):\n", len(scores))
	for _, s := range scores {
		displayName := criterionNames[s.CriterionIdentifier]
		if displayName == "" {
			displayName = "(unknown)"
		}
		fmt.Printf("  - %s: %s\n", s.CriterionIdentifier, displayName)
		fmt.Printf("    id: %s\n", s.ResourceID)
		if s.NumericValue.Valid {
			fmt.Printf("    numeric=%.2f", s.NumericValue.Float64)
		} else {
			fmt.Printf("    numeric=NULL")
		}
		if s.AIValue.Valid {
			fmt.Printf(", ai=%.2f", s.AIValue.Float64)
		}
		if s.TextValue.Valid && s.TextValue.String != "" {
			fmt.Printf(", text=%q", s.TextValue.String)
		}
		fmt.Printf(", na=%v, ai_scored=%v\n", s.NotApplicable, s.AIScored)
	}
}

func printCHScorecard(sc *CHScorecard) {
	fmt.Printf("\nScorecard:\n")
	fmt.Printf("  Scorecard ID:  %s\n", sc.ScorecardID)
	fmt.Printf("  Customer:      %s\n", sc.CustomerID)
	fmt.Printf("  Profile:       %s\n", sc.ProfileID)
	fmt.Printf("  Template:      %s (rev %s)\n", sc.TemplateID, sc.TemplateRev)
	fmt.Printf("  Agent User ID: %s\n", sc.AgentUserID)
	fmt.Printf("  Creator ID:    %s\n", sc.CreatorUserID)
	fmt.Printf("  Create Time:   %s\n", formatCHTime(sc.ScorecardCreateTime))
	fmt.Printf("  Last Update:   %s\n", formatCHTime(sc.ScorecardLastUpdate))
	fmt.Printf("  Submit Time:   %s\n", formatCHTime(sc.ScorecardSubmitTime))
	fmt.Printf("  Submitter ID:  %s\n", sc.SubmitterUserID)
	fmt.Printf("  Score:         %.2f\n", sc.Score)
	fmt.Printf("  Update Time:   %s\n", sc.UpdateTime.Format(time.RFC3339))
}

func printCHScores(scores []CHScore, criterionNames CriterionNameMap) {
	fmt.Printf("\nScores (%d):\n", len(scores))
	for _, s := range scores {
		displayName := criterionNames[s.CriterionID]
		if displayName == "" {
			displayName = "(unknown)"
		}
		fmt.Printf("  - %s: %s\n", s.CriterionID, displayName)
		fmt.Printf("    id: %s\n", s.ScoreID)
		fmt.Printf("    numeric=%.2f, ai=%.2f", s.NumericValue, s.AIValue)
		if s.TextValue != "" {
			fmt.Printf(", text=%q", s.TextValue)
		}
		fmt.Printf(", na=%v, ai_scored=%v\n", s.NotApplicable, s.AIScored)
		fmt.Printf("    submit_time=%s, update_time=%s\n",
			formatCHTime(s.ScorecardSubmitTime), s.UpdateTime.Format(time.RFC3339))
	}
}

func formatCHTime(t time.Time) string {
	if t.IsZero() || t.Year() <= 1970 {
		return "1970-01-01 (default/empty)"
	}
	return t.Format(time.RFC3339)
}

func validateSync(pgSc *PGScorecard, pgScores []PGScore, chSc *CHScorecard, chScores []CHScore, criterionNames CriterionNameMap) {
	allPassed := true
	fmt.Println()

	// Check 1: Both have data
	if pgSc == nil && chSc == nil {
		fmt.Println("WARN: Scorecard not found in both PostgreSQL and ClickHouse")
		return
	}
	if pgSc == nil {
		fmt.Println("FAIL: Scorecard not in PostgreSQL but exists in ClickHouse")
		return
	}
	if chSc == nil {
		fmt.Println("FAIL: Scorecard exists in PostgreSQL but NOT in ClickHouse")
		allPassed = false
	} else {
		fmt.Println("PASS: Scorecard exists in both PostgreSQL and ClickHouse")
	}

	// Check 2: Score count
	if len(pgScores) != len(chScores) {
		fmt.Printf("FAIL: Score count mismatch - PG: %d, CH: %d\n", len(pgScores), len(chScores))
		allPassed = false
	} else {
		fmt.Printf("PASS: Score count matches (%d)\n", len(pgScores))
	}

	// Check 3: Submitted state consistency
	if pgSc.SubmittedAt.Valid {
		pgSubmitTime := pgSc.SubmittedAt.Time
		fmt.Printf("\nScorecard is SUBMITTED in PostgreSQL (at %s)\n", pgSubmitTime.Format(time.RFC3339))

		if chSc != nil {
			if chSc.ScorecardSubmitTime.IsZero() || chSc.ScorecardSubmitTime.Year() <= 1970 {
				fmt.Println("FAIL: CH scorecard has empty/default scorecard_submit_time")
				allPassed = false
			} else if chSc.ScorecardSubmitTime.Before(pgSubmitTime.Add(-1 * time.Second)) {
				fmt.Printf("FAIL: CH submit_time (%s) is earlier than PG (%s)\n",
					chSc.ScorecardSubmitTime.Format(time.RFC3339),
					pgSubmitTime.Format(time.RFC3339))
				allPassed = false
			} else {
				fmt.Printf("PASS: CH scorecard_submit_time (%s) >= PG submitted_at\n",
					chSc.ScorecardSubmitTime.Format(time.RFC3339))
			}
		}

		// Check score-level submit times
		for _, chScore := range chScores {
			if chScore.ScorecardSubmitTime.IsZero() || chScore.ScorecardSubmitTime.Year() <= 1970 {
				fmt.Printf("FAIL: CH score %s has empty/default scorecard_submit_time\n", chScore.CriterionID)
				allPassed = false
			}
		}
	} else {
		fmt.Println("\nScorecard is NOT submitted in PostgreSQL")
		if chSc != nil && !chSc.ScorecardSubmitTime.IsZero() && chSc.ScorecardSubmitTime.Year() > 1970 {
			fmt.Printf("WARN: CH has submit_time (%s) but PG is not submitted\n",
				chSc.ScorecardSubmitTime.Format(time.RFC3339))
		}
	}

	// Check 4: Overall score
	if chSc != nil {
		if pgSc.Score.Valid {
			if pgSc.Score.Float64 != chSc.Score {
				fmt.Printf("FAIL: Overall score mismatch - PG: %.2f, CH: %.2f\n", pgSc.Score.Float64, chSc.Score)
				allPassed = false
			} else {
				fmt.Printf("PASS: Overall score matches (%.2f)\n", pgSc.Score.Float64)
			}
		}
	}

	// Check 5: Individual score values
	fmt.Println("\nComparing individual scores:")
	pgScoreMap := make(map[string]PGScore)
	for _, s := range pgScores {
		pgScoreMap[s.CriterionIdentifier] = s
	}

	for _, chScore := range chScores {
		pgScore, ok := pgScoreMap[chScore.CriterionID]
		displayName := criterionNames[chScore.CriterionID]
		if displayName == "" {
			displayName = "(unknown)"
		}
		criterionLabel := fmt.Sprintf("%s (%s)", chScore.CriterionID, displayName)

		if !ok {
			fmt.Printf("FAIL: CH has criterion %s not found in PG\n", criterionLabel)
			allPassed = false
			continue
		}

		issues := []string{}

		// Compare numeric value
		if pgScore.NumericValue.Valid {
			if pgScore.NumericValue.Float64 != chScore.NumericValue {
				issues = append(issues, fmt.Sprintf("numeric: PG=%.2f CH=%.2f", pgScore.NumericValue.Float64, chScore.NumericValue))
			}
		}

		// Compare AI value
		if pgScore.AIValue.Valid {
			if pgScore.AIValue.Float64 != chScore.AIValue {
				issues = append(issues, fmt.Sprintf("ai: PG=%.2f CH=%.2f", pgScore.AIValue.Float64, chScore.AIValue))
			}
		}

		// Compare text value
		pgText := ""
		if pgScore.TextValue.Valid {
			pgText = pgScore.TextValue.String
		}
		if pgText != chScore.TextValue {
			issues = append(issues, fmt.Sprintf("text: PG=%q CH=%q", pgText, chScore.TextValue))
		}

		// Compare not_applicable
		if pgScore.NotApplicable != chScore.NotApplicable {
			issues = append(issues, fmt.Sprintf("na: PG=%v CH=%v", pgScore.NotApplicable, chScore.NotApplicable))
		}

		// Compare ai_scored
		if pgScore.AIScored != chScore.AIScored {
			issues = append(issues, fmt.Sprintf("ai_scored: PG=%v CH=%v", pgScore.AIScored, chScore.AIScored))
		}

		if len(issues) > 0 {
			fmt.Printf("FAIL: %s - %s\n", criterionLabel, strings.Join(issues, ", "))
			allPassed = false
		} else {
			fmt.Printf("PASS: %s\n", criterionLabel)
		}
	}

	// Check for PG scores not in CH
	chScoreMap := make(map[string]bool)
	for _, s := range chScores {
		chScoreMap[s.CriterionID] = true
	}
	for _, pgScore := range pgScores {
		if !chScoreMap[pgScore.CriterionIdentifier] {
			displayName := criterionNames[pgScore.CriterionIdentifier]
			if displayName == "" {
				displayName = "(unknown)"
			}
			fmt.Printf("FAIL: PG has criterion %s (%s) not found in CH\n", pgScore.CriterionIdentifier, displayName)
			allPassed = false
		}
	}

	// Summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	if allPassed {
		fmt.Println("ALL VALIDATIONS PASSED - Data is synced correctly")
	} else {
		fmt.Println("SOME VALIDATIONS FAILED - Data sync issue detected")
	}
	fmt.Println(strings.Repeat("=", 60))
}

// parseScorecardName parses a full scorecard name like
// "customers/cox/profiles/sales/scorecards/019bd22d-fda5-745f-92df-9cb8f1644950"
// and returns customer, profile, scorecardID
func parseScorecardName(name string) (customer, profile, scorecardID string, err error) {
	// Format: customers/{customer}/profiles/{profile}/scorecards/{id}
	parts := strings.Split(name, "/")
	if len(parts) != 6 {
		return "", "", "", fmt.Errorf("invalid scorecard name format: expected 'customers/{customer}/profiles/{profile}/scorecards/{id}', got '%s'", name)
	}
	if parts[0] != "customers" || parts[2] != "profiles" || parts[4] != "scorecards" {
		return "", "", "", fmt.Errorf("invalid scorecard name format: expected 'customers/{customer}/profiles/{profile}/scorecards/{id}', got '%s'", name)
	}
	return parts[1], parts[3], parts[5], nil
}

func maskPassword(connStr string) string {
	// Simple password masking for display
	if idx := strings.Index(connStr, "password="); idx >= 0 {
		end := strings.Index(connStr[idx:], " ")
		if end == -1 {
			return connStr[:idx] + "password=***"
		}
		return connStr[:idx] + "password=***" + connStr[idx+end:]
	}
	return connStr
}
