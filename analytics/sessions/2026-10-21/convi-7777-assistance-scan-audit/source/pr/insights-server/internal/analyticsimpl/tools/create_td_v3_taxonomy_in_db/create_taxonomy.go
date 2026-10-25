// To run the script, first obtain a GRPC access token then pass it via LOCAL_GRPC_ACCESS_TOKEN,
// token can be obtained via cresta-cli or via Director admin app.
//
//		export LOCAL_GRPC_ACCESS_TOKEN=$(cresta-cli cresta-token voice-staging 2>/dev/null | grep "Access Token:" | awk '{print $3}')
//		go run ./insights-server/internal/analyticsimpl/tools/create_td_v3_taxonomy_in_db \
//		  -customer_id=<customer_id> \
//		  -profile_id=<profile_id> \
//	   -usecase_ids=<comma-separated-usecase-ids> \
//		  -cluster=voice-staging \
//		  -dry_run
//
// Endpoint format: grpc-cresta-api.<cluster>.internal.cresta.ai:443
// Clusters: chat-staging, chat-prod, voice-staging, voice-prod,
// GRPC access token can also be fetch via director admin app.
//
//	us-west-2-staging, us-west-2-prod, us-east-1-staging, us-east-1-prod
package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	grpcClient "github.com/cresta/shared-go/framework/client/grpc"
	"github.com/cresta/shared-go/middleware/rpcauth"
	"google.golang.org/grpc/metadata"
	fieldmaskpb "google.golang.org/protobuf/types/known/fieldmaskpb"
)

//go:embed sample_taxonomy.csv
var sampleTaxonomyCSV []byte

func main() {
	var customerID, profileID, cluster, usecaseIDs string
	var dryRun bool
	flag.StringVar(&customerID, "customer_id", "", "customer ID (required)")
	flag.StringVar(&profileID, "profile_id", "", "profile ID (required)")
	flag.StringVar(&usecaseIDs, "usecase_ids", "", "comma-separated list of use case IDs (required)")
	flag.StringVar(&cluster, "cluster", "", "cluster name (required, e.g. voice-staging, chat-prod)")
	flag.BoolVar(&dryRun, "dry_run", true, "if true, only call ListMoments to verify connectivity and skip upsert")
	flag.Parse()

	if customerID == "" || profileID == "" || cluster == "" || usecaseIDs == "" {
		log.Fatal("-customer_id, -profile_id, -cluster, and -usecase_ids are all required")
	}

	usecaseIDList := strings.Split(usecaseIDs, ",")

	for i, val := range usecaseIDList {
		usecaseIDList[i] = strings.TrimSpace(val)
	}

	endpoint := fmt.Sprintf("grpc-cresta-api.%s.internal.cresta.ai:443", cluster)

	profile := customerpb.ProfileName{CustomerID: customerID, ProfileID: profileID}

	ctx := context.Background()

	token := os.Getenv("LOCAL_GRPC_ACCESS_TOKEN")
	if token == "" {
		log.Fatal("LOCAL_GRPC_ACCESS_TOKEN is not set. Run: export LOCAL_GRPC_ACCESS_TOKEN=$(cresta-cli cresta-token <cluster> 2>/dev/null | grep \"Access Token:\" | awk '{print $3}')")
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "Authorization", "Bearer "+token)

	if err := run(ctx, endpoint, profile, usecaseIDList, dryRun); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, endpoint string, profile customerpb.ProfileName, usecaseIDList []string, dryRun bool) error {
	conn, err := grpcClient.NewClientConnection(
		ctx,
		endpoint,
		grpcClient.ConnectionWithRPCAuth(rpcauth.WithFromContext()),
		grpcClient.ConnectionUseTLS(true),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", endpoint, err)
	}
	defer conn.Close()

	client := momentpb.NewMomentServiceClient(conn)

	usecaseNames := []string{}
	for _, id := range usecaseIDList {
		usecaseNames = append(usecaseNames, customerpb.UsecaseName{CustomerID: profile.CustomerID, ProfileID: profile.ProfileID, UsecaseID: id}.String())
	}
	listResp, err := client.ListMoments(ctx, &momentpb.ListMomentsRequest{
		Parent: profile.String(),
		Filter: &momentpb.ListMomentsRequest_ListMomentsFilter{
			MomentTypes: []momentpb.Moment_Type{
				momentpb.Moment_TOPIC,
			},
			DetailedTypes: []momentpb.Moment_DetailedType{
				momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
			},
			States: []momentpb.Moment_State{
				momentpb.Moment_STATE_ACTIVE,
			},
			Usecases: usecaseNames,
		},
	})
	if err != nil {
		return fmt.Errorf("ListMoments failed (service stub not reachable): %w", err)
	}
	if len(listResp.GetMoments()) > 0 {
		return fmt.Errorf("ListMoments found active TDv3 moments for profile %s and usecases %v: upsert may create duplicate taxonomy. Found moments: %+v", profile.String(), usecaseIDList, listResp.GetMoments())
	}
	fmt.Printf("Verified no active TDv3 moments for profile %s and usecases %v. Proceeding with upsert...\n", profile.String(), usecaseIDList)

	if dryRun {
		fmt.Println("Dry run mode — skipping BatchUpsertMoments.")
		return nil
	}

	rows, err := readCSVRows()
	if err != nil {
		return fmt.Errorf("failed to read CSV: %w", err)
	}

	for i := range rows {
		rows[i].usecaseNames = usecaseNames
	}

	// Add a synthetic root row at level 0 as the parent of all level-1 moments,
	// This synthetic root node represents the whoel taxonomy.
	rootRow := csvRow{id: "0", name: "Dummy Root Node", usecaseNames: usecaseNames, description: "Root node for TD V3 taxonomy", level: 0}
	rows = append([]csvRow{rootRow}, rows...)

	return upsertTaxonomy(ctx, client, profile.String(), rows)
}

func upsertTaxonomy(ctx context.Context, client momentpb.MomentServiceClient, parent string, rows []csvRow) error {
	if err := validateTree(rows); err != nil {
		return fmt.Errorf("CSV validation failed: %w", err)
	}

	// Group rows by level for topological upsert.
	levelRows := map[int32][]csvRow{}
	for _, r := range rows {
		levelRows[r.level] = append(levelRows[r.level], r)
	}

	// csvID -> moment_template_id (populated after each level's upsert response).
	idMapping := map[string]string{}

	// Upsert root (level 0) first to obtain the root moment_template_id.
	rootRows := levelRows[0]
	if len(rootRows) != 1 {
		return fmt.Errorf("expected exactly 1 root row, got %d", len(rootRows))
	}
	rootMoment := buildMoment(rootRows[0])

	fmt.Printf("\nUpserting root moment to %s\n", parent)
	rootResp, err := client.BatchUpsertMoments(ctx, &momentpb.BatchUpsertMomentsRequest{
		Parent:  parent,
		Moments: []*momentpb.Moment{rootMoment},
	})
	if err != nil {
		return fmt.Errorf("BatchUpsertMoments (root): %w", err)
	}
	rootRespMoments := rootResp.GetMoments()
	if len(rootRespMoments) != 1 {
		return fmt.Errorf("BatchUpsertMoments (root): sent 1 moment but got %d back", len(rootRespMoments))
	}
	rootRespMoment := rootRespMoments[0]
	rootCSVID := rootRespMoment.GetTaxonomyReferenceId()
	if rootCSVID == "" {
		return fmt.Errorf("BatchUpsertMoments (root): response moment %q has empty TaxonomyReferenceId", rootRespMoment.GetName())
	}
	rootMomentName, err := momentpb.ParseMomentName(rootRespMoment.GetName())
	if err != nil {
		return fmt.Errorf("parse root moment name %q: %w", rootRespMoment.GetName(), err)
	}
	rootMomentTemplateID := rootMomentName.MomentID
	idMapping[rootCSVID] = rootMomentTemplateID
	fmt.Printf("  - [%s] %s (template_id=%s)\n", rootCSVID, rootRespMoment.GetDisplayName(), rootMomentTemplateID)

	// Upsert levels 1, 2, 3 with RootMomentTemplateId populated.
	for _, lvl := range []int32{1, 2, 3} {
		rowsAtLevel := levelRows[lvl]
		if len(rowsAtLevel) == 0 {
			continue
		}

		var moments []*momentpb.Moment
		for _, r := range rowsAtLevel {
			m := buildMoment(r)
			m.GetMomentData().GetTopicMomentData().GetConversationReasonV3().RootMomentTemplateId = rootMomentTemplateID
			parentCSVID := parentID(r.id)
			templateID, ok := idMapping[parentCSVID]
			if !ok {
				return fmt.Errorf("parent CSV id %q not found for row %q (was level %d upserted?)", parentCSVID, r.id, lvl-1)
			}
			m.ParentMomentTemplateId = templateID
			moments = append(moments, m)
		}

		fmt.Printf("\nUpserting %d level-%d moments to %s\n", len(moments), lvl, parent)
		upsertResp, err := client.BatchUpsertMoments(ctx, &momentpb.BatchUpsertMomentsRequest{
			Parent:  parent,
			Moments: moments,
		})
		if err != nil {
			return fmt.Errorf("BatchUpsertMoments (level %d): %w", lvl, err)
		}

		respMoments := upsertResp.GetMoments()
		if len(respMoments) != len(moments) {
			return fmt.Errorf("BatchUpsertMoments (level %d): sent %d moments but got %d back", lvl, len(moments), len(respMoments))
		}

		for _, m := range respMoments {
			csvID := m.GetTaxonomyReferenceId()
			if csvID == "" {
				return fmt.Errorf("BatchUpsertMoments (level %d): response moment %q has empty TaxonomyReferenceId", lvl, m.GetName())
			}
			momentName, err := momentpb.ParseMomentName(m.GetName())
			if err != nil {
				return fmt.Errorf("parse moment name %q: %w", m.GetName(), err)
			}
			idMapping[csvID] = momentName.MomentID
			fmt.Printf("  - [%s] %s (template_id=%s, parent=%s)\n", csvID, m.GetDisplayName(), momentName.MomentID, m.GetParentMomentTemplateId())
		}
	}

	// Update root moment with its own RootMomentTemplateId.
	rootRespMoment.GetMomentData().GetTopicMomentData().GetConversationReasonV3().RootMomentTemplateId = rootMomentTemplateID
	fmt.Printf("\nUpdating root moment with RootMomentTemplateId=%s\n", rootMomentTemplateID)
	if _, err := client.UpdateMoment(ctx, &momentpb.UpdateMomentRequest{
		Moment:     rootRespMoment,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"payload"}}, // Only update the payload to avoid
	}); err != nil {
		return fmt.Errorf("UpdateMoment (root): %w", err)
	}

	fmt.Printf("\nDone. Upserted %d moments total.\n", len(rows))
	return nil
}

type csvRow struct {
	usecaseNames []string
	id           string
	name         string
	description  string
	level        int32
}

func readCSVRows() ([]csvRow, error) {
	reader := csv.NewReader(bytes.NewReader(sampleTaxonomyCSV))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV has no data rows")
	}

	header := records[0]
	colIdx := make(map[string]int, len(header))
	for i, col := range header {
		colIdx[col] = i
	}
	for _, required := range []string{"id", "name", "description", "level"} {
		if _, ok := colIdx[required]; !ok {
			return nil, fmt.Errorf("CSV missing required column %q", required)
		}
	}

	var rows []csvRow //nolint:prealloc
	for i, record := range records[1:] {
		levelStr := record[colIdx["level"]]
		level, err := strconv.ParseInt(levelStr, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("row %d: invalid level %q: %w", i+2, levelStr, err)
		}
		rows = append(rows, csvRow{
			id:          record[colIdx["id"]],
			name:        record[colIdx["name"]],
			description: record[colIdx["description"]],
			level:       int32(level),
		})
	}
	return rows, nil
}

func buildMoment(r csvRow) *momentpb.Moment {
	return &momentpb.Moment{
		DisplayName:         r.name,
		Usecases:            r.usecaseNames,
		Description:         r.description,
		Type:                momentpb.Moment_TOPIC,
		DetailedType:        momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
		TaxonomyState:       momentpb.Moment_STATE_ACTIVE,
		Taxonomy:            fmt.Sprintf("td_v3_taxonomy_%s", r.id),
		TaxonomyReferenceId: r.id,
		AppTypes:            []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_TopicMomentData{
				TopicMomentData: &momentpb.TopicMomentData{
					Data: &momentpb.TopicMomentData_ConversationReasonV3_{
						ConversationReasonV3: &momentpb.TopicMomentData_ConversationReasonV3{
							Title:       r.name,
							Description: r.description,
							LevelIdx:    r.level,
							IsOther:     strings.HasPrefix(r.name, "Other"),
						},
					},
				},
			},
		},
	}
}

// validateTree checks that rows form a valid tree: no duplicate ids, exactly one
// root, every non-root row's parent exists, and levels are consistent with the
// id hierarchy.
func validateTree(rows []csvRow) error {
	idSet := map[string]csvRow{}
	for _, r := range rows {
		if _, exists := idSet[r.id]; exists {
			return fmt.Errorf("duplicate id %q", r.id)
		}
		idSet[r.id] = r
	}

	rootCount := 0
	for _, r := range rows {
		pid := parentID(r.id)
		if pid == "" {
			rootCount++
			continue
		}
		parent, ok := idSet[pid]
		if !ok {
			return fmt.Errorf("row %q references parent %q which does not exist", r.id, pid)
		}
		if parent.level != r.level-1 {
			return fmt.Errorf("row %q (level %d) has parent %q (level %d): expected parent level %d", r.id, r.level, pid, parent.level, r.level-1)
		}
	}

	if rootCount != 1 {
		return fmt.Errorf("expected exactly 1 root node, found %d", rootCount)
	}
	return nil
}

// parentID derives the parent CSV id from a child id.
// e.g. "1.2.3" -> "1.2", "1.2" -> "1", "1" -> "0" (root), "0" -> "" (no parent).
func parentID(id string) string {
	if id == "0" {
		return ""
	}
	lastDot := strings.LastIndex(id, ".")
	if lastDot < 0 {
		return "0"
	}
	return id[:lastDot]
}
