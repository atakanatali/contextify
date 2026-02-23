package steward

import "testing"

func TestRedactValue_RedactsSensitiveKeysAndPatterns(t *testing.T) {
	in := map[string]any{
		"authorization": "Bearer abcdefghijklmnop",
		"nested": map[string]any{
			"api_key": "sk_secretvalue123456",
			"note":    "safe",
		},
		"pem": "-----BEGIN PRIVATE KEY-----\nabc\n-----END PRIVATE KEY-----",
	}
	out := RedactValue(in).(map[string]any)
	if out["authorization"] != "REDACTED" {
		t.Fatalf("expected authorization key redacted, got %#v", out["authorization"])
	}
	nested, ok := out["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map")
	}
	if nested["api_key"] != "REDACTED" {
		t.Fatalf("expected nested api_key redacted, got %#v", nested["api_key"])
	}
	if out["_redacted"] != true {
		t.Fatalf("expected _redacted marker")
	}
	if _, ok := out["_redaction_reasons"]; !ok {
		t.Fatalf("expected redaction reasons")
	}
}

func TestRedactValue_RedactsBearerTokenPatternInStrings(t *testing.T) {
	in := map[string]any{"message": "Authorization: Bearer token_abcdefgh12345678"}
	out := RedactValue(in).(map[string]any)
	msg, _ := out["message"].(string)
	if msg == in["message"] {
		t.Fatalf("expected token redaction")
	}
	if out["_redacted"] != true {
		t.Fatalf("expected root redaction marker")
	}
}
