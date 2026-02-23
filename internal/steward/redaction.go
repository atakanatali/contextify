package steward

import (
	"regexp"
	"strings"
)

var (
	bearerTokenRe = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._\-]{8,}\b`)
	apiKeyLikeRe  = regexp.MustCompile(`(?i)\b(?:sk|api|token|key)_[a-z0-9]{8,}\b`)
	pemBlockRe    = regexp.MustCompile(`-----BEGIN [A-Z ]+-----[\s\S]*?-----END [A-Z ]+-----`)
)

var sensitiveKeyHints = []string{
	"password", "passwd", "secret", "token", "api_key", "apikey", "authorization", "auth", "private_key", "pem", "env",
}

func RedactValue(v any) any {
	out, _, _ := redactValue(v)
	return out
}

func redactValue(v any) (any, bool, []string) {
	switch x := v.(type) {
	case map[string]any:
		changed := false
		reasonsSet := map[string]struct{}{}
		out := make(map[string]any, len(x)+2)
		for k, val := range x {
			if shouldRedactKey(k) {
				out[k] = "REDACTED"
				changed = true
				reasonsSet["sensitive_key"] = struct{}{}
				continue
			}
			red, c, reasons := redactValue(val)
			out[k] = red
			if c {
				changed = true
			}
			for _, r := range reasons {
				reasonsSet[r] = struct{}{}
			}
		}
		if changed {
			out["_redacted"] = true
			out["_redaction_reasons"] = mapKeys(reasonsSet)
		}
		return out, changed, mapKeys(reasonsSet)
	case []any:
		changed := false
		reasonsSet := map[string]struct{}{}
		out := make([]any, len(x))
		for i, item := range x {
			red, c, reasons := redactValue(item)
			out[i] = red
			if c {
				changed = true
			}
			for _, r := range reasons {
				reasonsSet[r] = struct{}{}
			}
		}
		return out, changed, mapKeys(reasonsSet)
	case string:
		s, changed, reason := redactString(x)
		if !changed {
			return x, false, nil
		}
		return s, true, []string{reason}
	default:
		return v, false, nil
	}
}

func redactString(s string) (string, bool, string) {
	if pemBlockRe.MatchString(s) {
		return pemBlockRe.ReplaceAllString(s, "REDACTED_PEM_BLOCK"), true, "pem_block"
	}
	if bearerTokenRe.MatchString(s) {
		return bearerTokenRe.ReplaceAllString(s, "Bearer REDACTED_TOKEN"), true, "bearer_token"
	}
	if apiKeyLikeRe.MatchString(s) {
		return apiKeyLikeRe.ReplaceAllString(s, "REDACTED_API_KEY"), true, "api_key_pattern"
	}
	return s, false, ""
}

func shouldRedactKey(k string) bool {
	k = strings.ToLower(k)
	for _, hint := range sensitiveKeyHints {
		if strings.Contains(k, hint) {
			return true
		}
	}
	return false
}

func mapKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
