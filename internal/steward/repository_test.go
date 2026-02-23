package steward

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBackoffWithJitter_IncreasesWithAttempts(t *testing.T) {
	a := backoffWithJitter(1)
	b := backoffWithJitter(2)
	c := backoffWithJitter(3)

	if a < time.Second {
		t.Fatalf("attempt 1 backoff too small: %v", a)
	}
	if b < 2*time.Second {
		t.Fatalf("attempt 2 backoff too small: %v", b)
	}
	if c < 4*time.Second {
		t.Fatalf("attempt 3 backoff too small: %v", c)
	}
}

func TestRegistry_FallsBackToNoop(t *testing.T) {
	r := NewRegistry()
	ex, err := r.ExecutorFor("unknown_job")
	if err != nil {
		t.Fatalf("ExecutorFor returned error: %v", err)
	}
	res, err := ex.Execute(context.Background(), Job{JobType: "unknown_job"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if res == nil || res.Status != JobSucceeded {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestParseAutoMergeSuggestionPayload(t *testing.T) {
	id := "123e4567-e89b-12d3-a456-426614174000"
	got, err := parseAutoMergeSuggestionPayload(map[string]any{
		"suggestion_id":  id,
		"merge_strategy": "smart_merge",
	})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if got.SuggestionID.String() != id {
		t.Fatalf("unexpected suggestion id: %v", got.SuggestionID)
	}
	if got.MergeStrategy != "smart_merge" {
		t.Fatalf("unexpected merge strategy: %q", got.MergeStrategy)
	}
}

func TestSameProjectOrGlobal(t *testing.T) {
	a := "p1"
	b := "p1"
	c := "p2"
	if !sameProjectOrGlobal(nil, &a) {
		t.Fatalf("nil project should be allowed")
	}
	if !sameProjectOrGlobal(&a, &b) {
		t.Fatalf("same project should be allowed")
	}
	if sameProjectOrGlobal(&a, &c) {
		t.Fatalf("mismatched project should be blocked")
	}
}

func TestParseDerivePayload_UsesFallback(t *testing.T) {
	id := uuidMust("123e4567-e89b-12d3-a456-426614174000")
	got, err := parseDerivePayload(nil, []uuid.UUID{id})
	if err != nil {
		t.Fatalf("parseDerivePayload error: %v", err)
	}
	if len(got.SourceMemoryIDs) != 1 || got.SourceMemoryIDs[0] != id {
		t.Fatalf("unexpected source ids: %+v", got.SourceMemoryIDs)
	}
}

func uuidMust(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}
