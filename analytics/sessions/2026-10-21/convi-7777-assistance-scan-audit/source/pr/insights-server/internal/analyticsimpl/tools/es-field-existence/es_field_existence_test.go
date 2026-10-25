package main

import "testing"

func TestIsConversationV2Index(t *testing.T) {
	cases := []struct {
		name  string
		index string
		want  bool
	}{
		{name: "legacy concrete index", index: "cust-prof-conversations-v2", want: true},
		{name: "bare conversations-v2", index: "conversations-v2", want: true},
		{name: "rollover backing 000001", index: "cust-prof-conversations-v2-000001", want: true},
		{name: "rollover backing higher generation", index: "cust-prof-conversations-v2-000123", want: true},
		{name: "non-digit suffix is not a backing", index: "cust-prof-conversations-v2-embeddings", want: false},
		{name: "empty generation suffix", index: "cust-prof-conversations-v2-", want: false},
		{name: "v1 index", index: "cust-prof-conversations-v1", want: false},
		{name: "unrelated index", index: "cust-prof-messages", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// When
			got := isConversationV2Index(tc.index)

			// Then
			if got != tc.want {
				t.Errorf("isConversationV2Index(%q) = %v, want %v", tc.index, got, tc.want)
			}
		})
	}
}
