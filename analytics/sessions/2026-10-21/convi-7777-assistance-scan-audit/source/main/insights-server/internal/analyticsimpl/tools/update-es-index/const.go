package main

// We usually declare ES mapping in this file https://github.com/cresta/chat-ai/blob/master/greyparrot/insights/elastic_search/index.py.
// We could also copy all existing fields in the json string below.
// ES server will try to merge mapping from this json with existing mapping on server side.
const newMapping = `
{
    "properties": {
        "end_customer_id": {
            "type": "text",
            "fields": {
                "keyword": {
                    "type": "keyword",
                    "ignore_above": 256
                }
            }
        },
        "moment_annotations": {
            "type": "nested",
            "properties": {
                "payload_with_explicit_mapping": {
                    "properties": {
                        "metadata_number_value": {
                            "type": "double"
                        },
                        "outcome_number_value": {
                            "type": "double"
                        },
                        "llm_annotation_number_value": {
                            "type": "double"
                        }
                    }
                }
            }
        }
    }
}
	`

const newSettings = `
		{
			"index.mapping.total_fields.limit": 2000
		}
`
