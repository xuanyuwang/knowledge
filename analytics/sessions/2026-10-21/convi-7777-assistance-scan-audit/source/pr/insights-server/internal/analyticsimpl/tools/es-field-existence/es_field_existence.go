/*
List the indices that does NOT have a target field.

To run the script, run the following command in terminal
bazel run insights-server/internal/analyticsimpl/tools/es-field-existence:es_field_existence <es_address> <es_api_key> <field_name>
*/
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
)

type index struct {
	Index string `json:"index"`
}

func main() {
	// Set the Elasticsearch address and port
	esAddress := os.Args[1]
	esAPIKey := os.Args[2]
	fieldName := os.Args[3]

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

	filteredIndex := []string{}
	for _, index := range indexNames {
		hasField, err := hasFieldInMapping(esClient, index, fieldName)
		if err != nil {
			log.Fatalf("Filtering indexes: %s", err)
		}
		if !hasField {
			filteredIndex = append(filteredIndex, index)
		}
	}
	fmt.Printf("Index being scanned: \n%+v\n", indexNames)
	fmt.Printf("Index without the target field %v: \n%+v\n", fieldName, filteredIndex)
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
// this tool should inspect: either the legacy concrete index
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

func hasFieldInMapping(esClient *elasticsearch.Client, index string, field string) (bool, error) {
	res, err := esClient.Indices.GetMapping(
		esClient.Indices.GetMapping.WithIndex(index),
	)
	if err != nil {
		return false, fmt.Errorf("Error getting mapping: %s", err)
	}
	var mapping map[string]any
	if err := json.NewDecoder(res.Body).Decode(&mapping); err != nil {
		return false, fmt.Errorf("Error parsing mapping: %s", err)
	}

	defer res.Body.Close()
	return fieldExists(mapping, field), nil
}

func fieldExists(mapping map[string]any, field string) bool {
	ret := false
	for k, v := range mapping {
		if k == field {
			ret = true
		} else {
			if m, isMap := v.(map[string]any); isMap {
				ret = ret || fieldExists(m, field)
			}
		}
		if ret {
			break
		}
	}
	return ret
}
