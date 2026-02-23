package steward

import (
	"testing"
	"time"
)

func TestManagerLLMAllowed_RespectsBreakerCooldown(t *testing.T) {
	m := &Manager{}

	if !m.llmAllowed() {
		t.Fatal("expected llmAllowed when breaker is closed")
	}

	now := time.Now().UTC()
	cooldown := now.Add(30 * time.Second)
	m.breaker = circuitBreakerState{
		Open:          true,
		OpenedAt:      &now,
		CooldownUntil: &cooldown,
	}
	if m.llmAllowed() {
		t.Fatal("expected llm disallowed while breaker cooldown is active")
	}

	past := now.Add(-1 * time.Second)
	m.breaker.CooldownUntil = &past
	if !m.llmAllowed() {
		t.Fatal("expected llm allowed after breaker cooldown passes")
	}
}
