/*
To run the script, run the following command in terminal
bazel run insights-server/internal/analyticsimpl/tools/update-es-index:update_es_index <es_address> <es_api_key> <mapping|setting> true

close_before_update need to set to true for static setting:
For static setting, we need to close the index before update. For dynamic setting such as total_fields.limit, we can
just update without closing the index.
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

type index struct {
	Index string `json:"index"`
}

func main() {
	// Set the Elasticsearch address and port
	esAddress := os.Args[1]
	esAPIKey := os.Args[2]
	settingOrMapping := os.Args[3]
	dryRun := os.Args[4]

	cfg := elasticsearch.Config{
		Addresses: []string{
			esAddress,
		},
		APIKey: esAPIKey,
	}

	// Create the Elasticsearch client
	esClient, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating Elasticsearch client: %s", err)
	}

	indexNames, err := listIndices(esClient)
	if err != nil {
		log.Fatalf("Error listing existing indices: %s", err)
	}

	fmt.Printf("Index to be updated: %+v\n", indexNames)

	if dryRun != "false" {
		log.Println("Dry run, skip updating")
		return
	}

	switch settingOrMapping {
	case "setting":
		if err := updateSetting(esClient, indexNames); err != nil {
			log.Fatalf("update ES settings: %v", err)
		}
	case "mapping":
		if err := updateMapping(esClient, indexNames); err != nil {
			log.Fatalf("update ES mapping: %v", err)
		}
	default:
		log.Fatal("third flag need to be either 'setting' or 'mapping'")

	}

}

func listIndices(esClient *elasticsearch.Client) ([]string, error) {
	res, err := esClient.Cat.Indices(esClient.Cat.Indices.WithFormat("json"))
	if err != nil {
		return nil, fmt.Errorf("error getting response: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error in Cat.Indices response: %s", res.Status())
	}

	var indices []index
	if err := json.NewDecoder(res.Body).Decode(&indices); err != nil {
		return nil, fmt.Errorf("error decoding JSON response: %s", err)
	}

	indexNames := []string{}
	for _, index := range indices {
		if isConversationV2Index(index.Index) {
			indexNames = append(indexNames, index.Index)
		}
	}
	return indexNames, nil
}

// isConversationV2Index reports whether an ES index is a conversations-v2 index
// this tool should update: either the legacy concrete index
// (…-conversations-v2) or an alias-fronted rollover backing
// (…-conversations-v2-000001) created by the INSI-3834 sharding design. The
// generation suffix must be all digits so unrelated families such as
// …-conversations-v2-embeddings are not matched.
func isConversationV2Index(name string) bool {
	if strings.HasSuffix(name, "conversations-v2") {
		return true
	}
	const marker = "conversations-v2-"
	markerIndex := strings.LastIndex(name, marker)
	if markerIndex < 0 {
		return false
	}
	generation := name[markerIndex+len(marker):]
	if generation == "" {
		return false
	}
	for _, r := range generation {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func updateSetting(esClient *elasticsearch.Client, indexNames []string) error {
	fmt.Printf("Updating settings for index %v\n", indexNames)

	closeBeforeUpdate := os.Args[4]
	if closeBeforeUpdate == "true" {
		// Need to close an index before updating its setting.
		if err := closeIndex(esClient, indexNames); err != nil {
			return err
		}
		fmt.Println("Successfully closed index")

		defer func() {
			if err := openIndex(esClient, indexNames); err != nil {
				log.Fatalf("openIndex: %v", err)
			}
			fmt.Println("Successfully opened index")
		}()
	}
	// Create the API request.
	req := esapi.IndicesPutSettingsRequest{
		Index: indexNames,
		Body:  strings.NewReader(newSettings),
	}

	// Perform the request
	resp, err := req.Do(context.Background(), esClient)
	if err != nil {
		return fmt.Errorf("error updating index settings: %s", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("error from response: %s", resp.String())
	}
	return nil
}

func updateMapping(esClient *elasticsearch.Client, indexNames []string) error {
	fmt.Printf("Updating mapping for index %v\n", indexNames)
	// Call the _mapping API
	res, err := esClient.Indices.PutMapping(
		indexNames,
		strings.NewReader(newMapping),
		esClient.Indices.PutMapping.WithContext(context.Background()),
	)

	if err != nil {
		return fmt.Errorf("Error updating mapping: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("Error response from ES: %s", res.String())
	}

	fmt.Println("Mapping updated successfully")

	return nil
}

func closeIndex(esClient *elasticsearch.Client, indexNames []string) error {
	closeReq := esapi.IndicesCloseRequest{
		Index: indexNames,
	}
	closeRes, err := closeReq.Do(context.Background(), esClient)
	if err != nil {
		return fmt.Errorf("error closing index: %s", err)
	}
	defer closeRes.Body.Close()

	if closeRes.IsError() {
		return fmt.Errorf("close index response has error: %v", closeRes.String())
	}

	return nil
}

func openIndex(esClient *elasticsearch.Client, indexNames []string) error {
	openReq := esapi.IndicesOpenRequest{
		Index: indexNames,
	}
	openResp, err := openReq.Do(context.Background(), esClient)
	if err != nil {
		return fmt.Errorf("error opening index: %s", err)
	}
	defer openResp.Body.Close()

	if openResp.IsError() {
		return fmt.Errorf("open index response has error: %v", openResp.String())
	}

	return nil
}
