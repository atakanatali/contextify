package steward

import "testing"

func TestChoosePolicyProposal_RespectsMinSampleSize(t *testing.T) {
	p := choosePolicyProposal(tunerConfig{
		AutoMergeThreshold:      0.92,
		DerivationMinConfidence: 0.8,
		DerivationMinNovelty:    0.2,
		MinSampleSize:           100,
	}, &PolicyTuningEvidence{
		SampleSize:          10,
		AcceptedSuggest24h:  10,
		DismissedSuggest24h: 20,
	})
	if p != nil {
		t.Fatalf("expected no proposal, got %+v", p)
	}
}

func TestChoosePolicyProposal_AdjustsWithinBounds(t *testing.T) {
	p := choosePolicyProposal(tunerConfig{
		AutoMergeThreshold:      0.97,
		DerivationMinConfidence: 0.8,
		DerivationMinNovelty:    0.2,
		MinSampleSize:           20,
	}, &PolicyTuningEvidence{
		SampleSize:          100,
		SuccessRate24h:      0.95,
		AcceptedSuggest24h:  90,
		DismissedSuggest24h: 10,
	})
	if p == nil {
		t.Fatal("expected proposal")
	}
	if p.Key != "auto_merge_threshold" {
		t.Fatalf("expected auto_merge_threshold, got %s", p.Key)
	}
	if p.Next < 0.90 || p.Next > 0.97 {
		t.Fatalf("proposal out of bounds: %v", p.Next)
	}
	if p.Next != 0.965 {
		t.Fatalf("expected conservative decrease to 0.965, got %v", p.Next)
	}
}

func TestChoosePolicyProposal_PrioritizesSingleChangePerCycle(t *testing.T) {
	p := choosePolicyProposal(tunerConfig{
		AutoMergeThreshold:      0.92,
		DerivationMinConfidence: 0.8,
		DerivationMinNovelty:    0.2,
		MinSampleSize:           20,
	}, &PolicyTuningEvidence{
		SampleSize:          120,
		SuccessRate24h:      0.92,
		AcceptedSuggest24h:  5,
		DismissedSuggest24h: 35,
		AcceptedDerive24h:   1,
		SkippedDerive24h:    30,
	})
	if p == nil {
		t.Fatal("expected proposal")
	}
	if p.Key != "auto_merge_threshold" {
		t.Fatalf("expected auto_merge_threshold to be prioritized, got %s", p.Key)
	}
}
