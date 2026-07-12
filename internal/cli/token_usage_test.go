package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

func TestPrintTokenUsageHuman_UnknownCostNeverZeroDollars(t *testing.T) {
	agg := &session.Aggregation{
		ByModel: map[string]session.Usage{
			"mystery-model-9000": {
				Model: "mystery-model-9000", InputTokens: 1000, OutputTokens: 500,
				TotalTokens: 1500, SessionCount: 2, CostKnown: false,
			},
		},
		Total: session.Usage{
			Model: "total", InputTokens: 1000, OutputTokens: 500,
			TotalTokens: 1500, SessionCount: 2, CostKnown: false,
		},
		Period: "7d",
	}
	var buf bytes.Buffer
	printTokenUsageHuman(&buf, agg)
	out := buf.String()

	if !strings.Contains(out, "unknown") {
		t.Error("output should render unknown-cost models as 'unknown'")
	}
	if strings.Contains(out, "$   0.0000") || strings.Contains(out, "$0.0000") {
		t.Errorf("output must never show $0.00 for an unknown model:\n%s", out)
	}
	if !strings.Contains(out, "no pricing data") {
		t.Error("output should include the unknown-cost footnote")
	}
}

func TestPrintTokenUsageHuman_MixedKnownUnknownTotalIsLowerBound(t *testing.T) {
	agg := &session.Aggregation{
		ByModel: map[string]session.Usage{
			"claude-sonnet-4-6": {
				Model: "claude-sonnet-4-6", InputTokens: 1_000_000, OutputTokens: 0,
				TotalTokens: 1_000_000, SessionCount: 1, EstimatedCost: 3.0, CostKnown: true,
			},
			"mystery-model-9000": {
				Model: "mystery-model-9000", InputTokens: 100, OutputTokens: 50,
				TotalTokens: 150, SessionCount: 1, CostKnown: false,
			},
		},
		Total: session.Usage{
			Model: "total", InputTokens: 1_000_100, OutputTokens: 50,
			TotalTokens: 1_000_150, SessionCount: 2, EstimatedCost: 3.0, CostKnown: false,
		},
		Period: "7d",
	}
	var buf bytes.Buffer
	printTokenUsageHuman(&buf, agg)
	out := buf.String()

	if !strings.Contains(out, "≥$") {
		t.Errorf("total should render as a lower bound when some costs are unknown:\n%s", out)
	}
	if !strings.Contains(out, "$   3.0000") {
		t.Errorf("known model cost should still be shown:\n%s", out)
	}
}

func TestPrintTokenUsageHuman_AllKnown_NoFootnote(t *testing.T) {
	agg := &session.Aggregation{
		ByModel: map[string]session.Usage{
			"gpt-4o": {
				Model: "gpt-4o", InputTokens: 100, OutputTokens: 50, TotalTokens: 150,
				SessionCount: 1, EstimatedCost: 0.00075, CostKnown: true,
			},
		},
		Total: session.Usage{
			Model: "total", InputTokens: 100, OutputTokens: 50, TotalTokens: 150,
			SessionCount: 1, EstimatedCost: 0.00075, CostKnown: true,
		},
		Period: "7d",
	}
	var buf bytes.Buffer
	printTokenUsageHuman(&buf, agg)
	out := buf.String()

	if strings.Contains(out, "unknown") {
		t.Errorf("fully priced output should not mention unknown:\n%s", out)
	}
}
