package analyticsimpl

import (
	"strings"
	"testing"

	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
)

// TestMetadataValuesClickhouseQuery_ParameterizesStringContains is the SECH1-48
// regression test. It calls the pure query builder with a SQL-injection payload
// and asserts the value is bound as a query parameter (present in args, absent
// from the query text), so it can never break out of the LIKE string literal.
//
// The existing RetrieveMetadataValuesTestSuite golden-file comparison cannot
// prove this: it recombines args back into "?" slots before diffing
// (v2CombineQueryAndArgs), which would mask a regression to interpolation.
func TestMetadataValuesClickhouseQuery_ParameterizesStringContains(t *testing.T) {
	payload := "abc'||toString(cityHash64(1))||'"

	query, args, err := metadataValuesClickhouseQuery(
		metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
		"moment-1", payload, nil /*usecaseConditionAndArgs*/, false, /*numericMinMaxOnly*/
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The LIKE filter must use a placeholder, not the interpolated value.
	if !strings.Contains(query, "metadata_string_value LIKE ?") {
		t.Errorf("expected parameterized `LIKE ?` placeholder in query, got:\n%s", query)
	}
	// The payload must not appear verbatim in the query text — that would mean
	// it was interpolated and is therefore injectable.
	if strings.Contains(query, "cityHash64") || strings.Contains(query, "throwIf") || strings.Contains(query, "||") {
		t.Errorf("payload must not appear in query text (injection not parameterized), got:\n%s", query)
	}

	// The LIKE pattern (with caller-supplied `%` wildcards) must travel in args,
	// where the ClickHouse driver escapes it.
	wantArg := "%" + payload + "%"
	found := false
	for _, a := range args {
		if s, ok := a.(string); ok && s == wantArg {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected LIKE pattern %q in args, got args: %#v", wantArg, args)
	}
}

// TestMetadataValuesClickhouseQuery_ParameterizesMomentTemplateID is the SECH1-49
// regression test (defense-in-depth). momentTemplateID is server-validated via
// BatchGetMoments and not directly caller-controlled, so this is hardening, not
// an exploitable bug. The test still asserts the value is bound as a query
// parameter (present in args, absent from the query text) so no quoted-value
// interpolation remains in the file.
//
// The LIKE clause (metadataStringContains) is parameterized separately by
// SECH1-48 (PR #31819, merged); this test only covers the momentTemplateID
// sites at lines 40 and 76.
func TestMetadataValuesClickhouseQuery_ParameterizesMomentTemplateID(t *testing.T) {
	// Main query path (string type, no substring filter, no usecase filter).
	query, args, err := metadataValuesClickhouseQuery(
		metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
		"moment-1", "" /*metadataStringContains*/, nil, /*usecaseConditionAndArgs*/
		false, /*numericMinMaxOnly*/
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(query, "moment_template_id = ?") {
		t.Errorf("expected parameterized `moment_template_id = ?` in query, got:\n%s", query)
	}
	if strings.Contains(query, "moment-1") {
		t.Errorf("momentTemplateID must not appear verbatim in query text, got:\n%s", query)
	}
	if len(args) == 0 || args[0] != "moment-1" {
		t.Errorf("expected momentTemplateID to be the first arg, got args: %#v", args)
	}

	// Numeric min-max path (the other interpolation site).
	mmQuery, mmArgs, err := metadataValuesClickhouseQuery(
		metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
		"moment-9", "" /*metadataStringContains*/, nil, /*usecaseConditionAndArgs*/
		true, /*numericMinMaxOnly*/
	)
	if err != nil {
		t.Fatalf("unexpected error (min-max): %v", err)
	}
	if !strings.Contains(mmQuery, "moment_template_id = ?") {
		t.Errorf("expected parameterized `moment_template_id = ?` in min-max query, got:\n%s", mmQuery)
	}
	if strings.Contains(mmQuery, "moment-9") {
		t.Errorf("momentTemplateID must not appear verbatim in min-max query text, got:\n%s", mmQuery)
	}
	if len(mmArgs) == 0 || mmArgs[0] != "moment-9" {
		t.Errorf("expected momentTemplateID to be the first arg in min-max path, got args: %#v", mmArgs)
	}
}
