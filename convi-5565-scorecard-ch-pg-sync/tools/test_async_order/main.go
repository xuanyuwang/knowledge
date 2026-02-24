package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	coachingpb "github.com/cresta/cresta-proto/gen/go/cresta/v1/coaching"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// Test configuration
type Config struct {
	GRPCHost         string
	Customer         string
	Profile          string
	TemplateName     string
	AgentUserName    string
	ConversationName string
	Criterion1       string
	Criterion2       string
	Iterations       int
	WaitSeconds      int
	APIDelayMs       int // Delay between Update and Submit API calls
	CHHost           string
	CHPort           int
	CHUser           string
	CHPass           string
	CHDatabase       string
}

// Test result
type TestResult struct {
	Iteration   int
	ScorecardID string
	Passed      bool
	Error       string
	CreateTime  time.Duration
	UpdateTime  time.Duration
	SubmitTime  time.Duration
	VerifyTime  time.Duration
}

func main() {
	iterations := flag.Int("iterations", 50, "Number of test iterations")
	waitSeconds := flag.Int("wait", 2, "Seconds to wait before verifying ClickHouse")
	apiDelayMs := flag.Int("api-delay", 10, "Milliseconds to wait between Update and Submit API calls")
	flag.Parse()

	config := Config{
		GRPCHost:         "grpc.chat-staging.cresta.ai:443",
		Customer:         "cox",
		Profile:          "sales",
		TemplateName:     "customers/cox/profiles/sales/scorecardTemplates/c1876249-0bfd-4aaf-a316-132a54a7e70c@7aad25a1",
		AgentUserName:    "customers/cox/users/8c8b7449f1497e86",
		ConversationName: "customers/cox/profiles/sales/conversations/019bc327-a9ed-7020-84ea-d13ed553f904",
		Criterion1:       "4583ff48-c7b5-4d73-aa39-3911d4f9f95d",
		Criterion2:       "562116d8-6b46-49f8-bcf1-33668375698e",
		Iterations:       *iterations,
		WaitSeconds:      *waitSeconds,
		APIDelayMs:       *apiDelayMs,
		CHHost:           "clickhouse-conversations.chat-staging.internal.cresta.ai",
		CHPort:           9440,
		CHUser:           os.Getenv("CH_USER"),
		CHPass:           os.Getenv("CH_PASS"),
		CHDatabase:       "cox_sales",
	}

	fmt.Println("============================================================")
	fmt.Println("CONVI-5565 Async Order Test")
	fmt.Println("============================================================")
	fmt.Printf("Iterations: %d\n", config.Iterations)
	fmt.Printf("Wait time: %ds\n", config.WaitSeconds)
	fmt.Printf("API delay: %dms\n", config.APIDelayMs)
	fmt.Printf("GRPC Host: %s\n", config.GRPCHost)
	fmt.Println("============================================================")
	fmt.Println()

	// Get auth token
	fmt.Println("Getting auth token...")
	token, err := getAuthToken()
	if err != nil {
		fmt.Printf("ERROR: Failed to get auth token: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Token acquired")
	fmt.Println()

	// Connect to gRPC
	fmt.Println("Connecting to gRPC...")
	conn, err := grpc.Dial(config.GRPCHost,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)
	if err != nil {
		fmt.Printf("ERROR: Failed to connect to gRPC: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	client := coachingpb.NewCoachingServiceClient(conn)
	fmt.Println("gRPC connected")

	// Connect to ClickHouse
	fmt.Println("Connecting to ClickHouse...")
	chConn, err := connectClickHouse(config)
	if err != nil {
		fmt.Printf("ERROR: Failed to connect to ClickHouse: %v\n", err)
		os.Exit(1)
	}
	defer chConn.Close()
	fmt.Println("ClickHouse connected")
	fmt.Println()

	// Run tests
	fmt.Println("Starting test iterations...")
	fmt.Println()

	results := make([]TestResult, 0, config.Iterations)
	passed := 0
	failed := 0

	for i := 1; i <= config.Iterations; i++ {
		result := runTestIteration(i, client, chConn, token, config)
		results = append(results, result)

		if result.Passed {
			passed++
			fmt.Printf("  [PASS] Iteration %d: %s\n", i, result.ScorecardID)
		} else {
			failed++
			fmt.Printf("  [FAIL] Iteration %d: %s - %s\n", i, result.ScorecardID, result.Error)
		}
	}

	// Print summary
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("TEST SUMMARY")
	fmt.Println("============================================================")
	fmt.Printf("Total iterations: %d\n", config.Iterations)
	fmt.Printf("Passed: %d\n", passed)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Success rate: %.2f%%\n", float64(passed)*100/float64(config.Iterations))
	fmt.Println()

	if failed > 0 {
		fmt.Println("Failed iterations:")
		for _, r := range results {
			if !r.Passed {
				fmt.Printf("  - Iteration %d: %s (%s)\n", r.Iteration, r.ScorecardID, r.Error)
			}
		}
		fmt.Println()
	}

	fmt.Println("============================================================")

	if failed > 0 {
		os.Exit(1)
	}
}

func getAuthToken() (string, error) {
	cmd := exec.Command("cresta-cli", "cresta-token", "chat-staging", "cox", "--bearer")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func connectClickHouse(config Config) (clickhouse.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", config.CHHost, config.CHPort)},
		Auth: clickhouse.Auth{
			Database: config.CHDatabase,
			Username: config.CHUser,
			Password: config.CHPass,
		},
		TLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	})
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(context.Background()); err != nil {
		return nil, err
	}

	return conn, nil
}

func runTestIteration(iteration int, client coachingpb.CoachingServiceClient, chConn clickhouse.Conn, token string, config Config) TestResult {
	result := TestResult{
		Iteration: iteration,
	}

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", token)
	parent := fmt.Sprintf("customers/%s/profiles/%s", config.Customer, config.Profile)

	// Step 1: Create scorecard
	createStart := time.Now()
	createResp, err := client.CreateScorecard(ctx, &coachingpb.CreateScorecardRequest{
		Parent: parent,
		Scorecard: &coachingpb.Scorecard{
			TemplateName:     config.TemplateName,
			AgentUserName:    config.AgentUserName,
			ConversationName: config.ConversationName,
			Scores: []*coachingpb.Score{
				{CriterionId: config.Criterion1, NumericValue: floatPtr(1)},
				{CriterionId: config.Criterion2, NumericValue: floatPtr(0)},
			},
			SubmissionSource: coachingpb.ScorecardSubmissionSource_SCORECARD_SUBMISSION_SOURCE_CLOSED_CONVERSATIONS,
		},
	})
	result.CreateTime = time.Since(createStart)

	if err != nil {
		result.Error = fmt.Sprintf("Create failed: %v", err)
		return result
	}

	scorecardName := createResp.Scorecard.Name
	result.ScorecardID = scorecardName[strings.LastIndex(scorecardName, "/")+1:]

	// Step 2 & 3: Update and Submit RAPIDLY (concurrent)
	var wg sync.WaitGroup
	var updateErr, submitErr error

	wg.Add(2)

	// Update scorecard
	go func() {
		defer wg.Done()
		updateStart := time.Now()
		_, updateErr = client.UpdateScorecard(ctx, &coachingpb.UpdateScorecardRequest{
			Scorecard: &coachingpb.Scorecard{
				Name: scorecardName,
				Scores: []*coachingpb.Score{
					{CriterionId: config.Criterion1, NumericValue: floatPtr(0)},
					{CriterionId: config.Criterion2, NumericValue: floatPtr(1)},
				},
			},
		})
		result.UpdateTime = time.Since(updateStart)
	}()

	// Submit scorecard (with configurable delay to control timing)
	var submitResp *coachingpb.SubmitScorecardResponse
	go func() {
		defer wg.Done()
		time.Sleep(time.Duration(config.APIDelayMs) * time.Millisecond)
		submitStart := time.Now()
		submitResp, submitErr = client.SubmitScorecard(ctx, &coachingpb.SubmitScorecardRequest{
			Name: scorecardName,
		})
		result.SubmitTime = time.Since(submitStart)
	}()

	wg.Wait()

	if updateErr != nil {
		result.Error = fmt.Sprintf("Update failed: %v", updateErr)
		// Still try to reset
		resetScorecard(ctx, client, scorecardName)
		return result
	}

	if submitErr != nil {
		result.Error = fmt.Sprintf("Submit failed: %v", submitErr)
		resetScorecard(ctx, client, scorecardName)
		return result
	}

	// Check if submit response shows submitted state
	if submitResp != nil && submitResp.Scorecard != nil {
		if submitResp.Scorecard.SubmitTime == nil {
			fmt.Printf("    [DEBUG] Submit API returned but SubmitTime is nil for %s\n", result.ScorecardID)
		}
	}

	// Step 4: Wait for async work to complete
	time.Sleep(time.Duration(config.WaitSeconds) * time.Second)

	// Step 5: Verify ClickHouse data
	verifyStart := time.Now()
	verifyErr := verifyClickHouse(chConn, result.ScorecardID)
	result.VerifyTime = time.Since(verifyStart)

	if verifyErr != nil {
		result.Error = verifyErr.Error()
		// DON'T reset failed scorecards so we can investigate them
		fmt.Printf("    [DEBUG] Keeping failed scorecard for investigation: %s\n", scorecardName)
		return result
	}

	// Step 6: Reset scorecard (only on success)
	resetScorecard(ctx, client, scorecardName)

	result.Passed = true
	return result
}

func verifyClickHouse(conn clickhouse.Conn, scorecardID string) error {
	ctx := context.Background()

	// Check scorecard table
	var submitTime time.Time
	err := conn.QueryRow(ctx, `
		SELECT scorecard_submit_time
		FROM scorecard_d FINAL
		WHERE scorecard_id = $1
	`, scorecardID).Scan(&submitTime)

	if err != nil {
		return fmt.Errorf("scorecard not found in CH: %v", err)
	}

	if submitTime.IsZero() || submitTime.Year() <= 1970 {
		return fmt.Errorf("invalid scorecard_submit_time: %v", submitTime)
	}

	// Check score table
	rows, err := conn.Query(ctx, `
		SELECT criterion_id, scorecard_submit_time
		FROM score_d FINAL
		WHERE scorecard_id = $1
	`, scorecardID)
	if err != nil {
		return fmt.Errorf("failed to query scores: %v", err)
	}
	defer rows.Close()

	scoreCount := 0
	for rows.Next() {
		var criterionID string
		var scoreSubmitTime time.Time
		if err := rows.Scan(&criterionID, &scoreSubmitTime); err != nil {
			return fmt.Errorf("failed to scan score: %v", err)
		}

		if scoreSubmitTime.IsZero() || scoreSubmitTime.Year() <= 1970 {
			return fmt.Errorf("invalid score submit_time for %s: %v", criterionID, scoreSubmitTime)
		}
		scoreCount++
	}

	if scoreCount == 0 {
		return fmt.Errorf("no scores found in CH")
	}

	return nil
}

func resetScorecard(ctx context.Context, client coachingpb.CoachingServiceClient, name string) {
	_, _ = client.ResetScorecard(ctx, &coachingpb.ResetScorecardRequest{
		Name: name,
	})
}

func floatPtr(f float32) *float32 {
	return &f
}
