package llm

import "testing"

func TestParseAndValidateDecision_Valid(t *testing.T) {
	d, err := ParseAndValidateDecision([]byte(`{
		"is_duplicate": true,
		"has_conflict": false,
		"decision": "merge",
		"confidence": 0.93,
		"recommended_strategy": "smart_merge",
		"merged_title": "Merged",
		"merged_content": "Body",
		"reason_codes": ["high_similarity"]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Decision != "merge" || d.Confidence != 0.93 {
		t.Fatalf("unexpected decision: %+v", d)
	}
}

func TestParseAndValidateDecision_InvalidDecision(t *testing.T) {
	if _, err := ParseAndValidateDecision([]byte(`{"decision":"bad","confidence":0.5}`)); err == nil {
		t.Fatalf("expected validation error")
	}
}
