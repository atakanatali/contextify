package steward

import (
	"testing"
	"time"
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
	res, err := ex.Execute(t.Context(), Job{JobType: "unknown_job"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if res == nil || res.Status != JobSucceeded {
		t.Fatalf("unexpected result: %+v", res)
	}
}
