/*
To run the script, run the following command in terminal
bazel run //insights-server/internal/analyticsimpl/tools/runtime-keyword-matching:runtime_keyword_matching <esAddress> <esAPIKey> <index_name_prefix> <concurrent_count> <number_of_runs> <number_indices_in_pool>

About index_name_prefix and number_indices_in_pool:
  - when number_indices_in_pool > 1, then the index names being used are `<index_name_prefix>i`, where i is 0,1,..., number_indices_in_pool-1
  - if number_indices_in_pool == 1, the index name being use is <index_name_prefix>

The value of esAddress and esAPIKey can be found in https://www.notion.so/cresta/Yahui-s-Insights-Server-Note-7828cc0e2e334f738d72cda5abe1bf21#43522a233d544de8b367408d4eda9369

eg1: using just 1 index:
bazel run //insights-server/internal/analyticsimpl/tools/runtime-keyword-matching:runtime_keyword_matching <esAddress> <esAPIKey>  100 10 1

eg2: using multiple indices:
bazel run //insights-server/internal/analyticsimpl/tools/runtime-keyword-matching:runtime_keyword_matching <esAddress> <esAPIKey> caiyi-zhu-test-v 100 10 10
*/
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cresta/go-servers/shared/utils"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func main() {
	// Set the Elasticsearch address and port
	esAddress := os.Args[1]
	esAPIKey := os.Args[2]
	esIndexNamePrefix := os.Args[3]
	concurrenyCount, err := strconv.Atoi(os.Args[4])
	if err != nil {
		log.Fatalf("Concurrency number is not valid: %v", err)
	}
	numOfRuns, err := strconv.Atoi(os.Args[5])
	if err != nil {
		log.Fatalf("Number of runs is not valid: %v", err)
	}
	numOfIndicesInPool, err := strconv.Atoi(os.Args[6])
	if err != nil {
		log.Fatalf("Number of indices in pool is not valid: %v", err)
	}

	// Create the Elasticsearch client config
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

	// Delete all existing documents.
	for i := 0; i < numOfIndicesInPool; i++ {
		esIndexName := fmt.Sprintf("%s%d", esIndexNamePrefix, i)
		if numOfIndicesInPool == 1 {
			esIndexName = esIndexNamePrefix
		}
		if err := deleteAll(esIndexName, esClient); err != nil {
			log.Fatalf("deleteAll(): %v", err)
		}
	}

	var wg sync.WaitGroup
	var notFoundCount atomic.Int64
	insertLatencies := make([]int64, concurrenyCount*numOfRuns)
	searchLatencies := make([]int64, concurrenyCount*numOfRuns)
	totalLatencies := make([]int64, concurrenyCount*numOfRuns)

	ctx := context.Background()
	for i := 0; i < numOfRuns; i++ {
		log.Printf("Run number %d", i+1)
		for j := 0; j < concurrenyCount; j++ {
			j := j
			esIndexName := fmt.Sprintf("%s%d", esIndexNamePrefix, j%numOfIndicesInPool)
			if numOfIndicesInPool == 1 {
				esIndexName = esIndexNamePrefix
			}
			idx := i*concurrenyCount + j
			wg.Add(1)

			go utils.AvoidPanic(ctx, func() {
				defer wg.Done()

				// Define the index name, document ID, and document body
				documentID := fmt.Sprintf("id-%d", idx)
				messageText := "this is too expensive, can you provide some discounts, this is too expensive, can you provide some discounts, this is too expensive, can you provide some discounts, this is too expensive, can you provide some discounts, this is too expensive, can you provide some discounts, this is too expensive, can you provide some discounts"
				keyword := "expensive"

				// Add document to ES index.
				insertStartTime := time.Now()
				if err := insertDocument(messageText, documentID, esIndexName, esClient); err != nil {
					log.Fatalf("insertDocument(): %v", err)
				}
				insertLatencies[idx] = time.Since(insertStartTime).Milliseconds()

				// Search for the newly added document.
				searchStartTime := time.Now()
				found, err := searchDocument(documentID, keyword, esIndexName, esClient)
				if err != nil {
					log.Fatalf("searchDocument(): %v", err)
				}
				if !found {
					notFoundCount.Add(1)
				}
				totalLatencies[idx] = time.Since(insertStartTime).Milliseconds()
				searchLatencies[idx] = time.Since(searchStartTime).Milliseconds()
			})()
		}
		wg.Wait()
	}
	log.Println("The following prints insert latency, search latency, and total latency (in millisec)")
	percentileStats(insertLatencies)
	percentileStats(searchLatencies)
	percentileStats(totalLatencies)
	fmt.Printf("Counts of keyword not found in document: %d\n", notFoundCount.Load())
}

func searchDocument(documentID, keyword, esIndexName string, esClient *elasticsearch.Client) (bool, error) {
	queryBody := fmt.Sprintf(`{
		"query": {
		  "bool": {
			"filter": [
			  {
				"terms": {
				  "_id": [
					"%s"
				  ]
				}
			  },
			  {
				"multi_match": {
				  "query": "%s",
				  "type": "phrase",
				  "fields": [
					"message_text"
				  ]
				}
			  }
			]
		  }
		}
	  }`, documentID, keyword)
	// Perform a sample search request
	resp, err := esClient.Search(
		esClient.Search.WithContext(context.Background()),
		esClient.Search.WithIndex(esIndexName),
		esClient.Search.WithBody(strings.NewReader(queryBody)),
		esClient.Search.WithSize(10),
	)
	if err != nil {
		return false, fmt.Errorf("error performing search request: %s", err)
	}
	if resp.IsError() {
		return false, fmt.Errorf("search response has error: %v", resp.String())
	}
	defer resp.Body.Close()

	foundMatch := hasMatchedDoc(resp)
	return foundMatch, nil
}

func insertDocument(messageText, documentID, esIndexName string, esClient *elasticsearch.Client) error {
	documentBody := fmt.Sprintf(`{"message_text" : "%s"}`, messageText)

	// Create the request object
	req := esapi.IndexRequest{
		Index:      esIndexName,
		DocumentID: documentID,
		Body:       strings.NewReader(documentBody),
		Refresh:    "wait_for",
	}

	// Perform the request
	resp, err := req.Do(context.Background(), esClient)
	if err != nil {
		return fmt.Errorf("indexing document: %s", err)
	}
	if resp.IsError() {
		return fmt.Errorf("insert response has error: %v", resp.String())
	}
	defer resp.Body.Close()
	return nil
}

func hasMatchedDoc(resp *esapi.Response) bool {
	matchedString := "\"value\":1"
	respStr := ""
	if b, err := io.ReadAll(resp.Body); err == nil {
		respStr = string(b)
	}
	return strings.Contains(respStr, matchedString)
}

//nolint:unused
func deleteDocument(documentID, indexName string, esClient *elasticsearch.Client) error {
	// Create the delete request
	req := esapi.DeleteRequest{
		Index:      indexName,
		DocumentID: documentID,
	}
	// Perform the request
	resp, err := req.Do(context.Background(), esClient)
	if err != nil {
		return fmt.Errorf("deleting document: %s", err)
	}
	if resp.IsError() {
		return fmt.Errorf("delete response has error: %s", resp.String())
	}
	defer resp.Body.Close()
	return nil
}

func deleteAll(indexName string, esClient *elasticsearch.Client) error {
	queryBody := `{
		"query" : {
			"match_all" : {}
		}
	}`
	resp, err := esClient.DeleteByQuery([]string{indexName}, strings.NewReader(queryBody))
	if err != nil {
		return fmt.Errorf("delete all documents failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("delete all documents response has error: %s", resp.String())
	}
	return nil
}

func percentileStats(vals []int64) {
	sort.Slice(vals, func(i, j int) bool {
		return vals[i] >= vals[j]
	})
	p50 := vals[len(vals)/2]
	p90 := vals[len(vals)/10]
	p99 := vals[len(vals)/100]
	log.Printf("p50: %v, p90: %v, p99: %v", p50, p90, p99)
}
