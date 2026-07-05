package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

func sampleAggregation() *session.Aggregation {
	return &session.Aggregation{
		ByModel: map[string]session.Usage{
			"claude-sonnet-4-6": {
				Model:               "claude-sonnet-4-6",
				InputTokens:         1000,
				OutputTokens:        500,
				CacheReadTokens:     8000,
				CacheCreationTokens: 2000,
				TotalTokens:         1500,
				EstimatedCost:       0.05,
				SessionCount:        2,
			},
		},
		Total: session.Usage{
			Model:               "total",
			InputTokens:         1000,
			OutputTokens:        500,
			CacheReadTokens:     8000,
			CacheCreationTokens: 2000,
			TotalTokens:         1500,
			EstimatedCost:       0.05,
			SessionCount:        2,
		},
		MaxSessionTokens: 1200,
		Period:           "7d",
	}
}

func TestPrintTokenUsageHuman_CacheColumns(t *testing.T) {
	var buf bytes.Buffer
	printTokenUsageHuman(&buf, sampleAggregation())
	out := buf.String()

	for _, want := range []string{"Cache-Rd", "Cache-Wr", "8000", "2000", "claude-sonnet-4-6", "TOTAL"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintTokenUsageHuman_NoSessionsMentionsClaude(t *testing.T) {
	var buf bytes.Buffer
	printTokenUsageHuman(&buf, &session.Aggregation{ByModel: map[string]session.Usage{}, Period: "7d"})
	if !strings.Contains(buf.String(), "Claude Code") {
		t.Errorf("empty-state output should mention Claude Code:\n%s", buf.String())
	}
}

func TestPrintBudgetFooter(t *testing.T) {
	daily := &session.Aggregation{
		Total:            session.Usage{TotalTokens: 143_000},
		MaxSessionTokens: 12_500,
	}
	tests := []struct {
		name        string
		usage       domain.UsageConfig
		daily       *session.Aggregation
		wantParts   []string
		wantAbsent  []string
		wantNothing bool
	}{
		{
			name:  "within budget",
			usage: domain.UsageConfig{DailyTokenBudget: 200_000, SessionTokenBudget: 50_000, Enforcement: "warn"},
			daily: daily,
			wantParts: []string{
				"Daily budget:   143.0k / 200.0k (71%)",
				"Session budget: largest session 12.5k / 50.0k (25%)",
			},
			wantAbsent: []string{"over budget"},
		},
		{
			name:      "over budget",
			usage:     domain.UsageConfig{DailyTokenBudget: 100_000, Enforcement: "warn"},
			daily:     daily,
			wantParts: []string{"Daily budget:   143.0k / 100.0k (143%) — over budget"},
		},
		{
			name:        "enforcement off suppresses footer",
			usage:       domain.UsageConfig{DailyTokenBudget: 100_000, Enforcement: "off"},
			daily:       daily,
			wantNothing: true,
		},
		{
			name:        "no budgets configured suppresses footer",
			usage:       domain.UsageConfig{Enforcement: "warn"},
			daily:       daily,
			wantNothing: true,
		},
		{
			name:        "nil aggregation suppresses footer",
			usage:       domain.UsageConfig{DailyTokenBudget: 100_000, Enforcement: "warn"},
			daily:       nil,
			wantNothing: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printBudgetFooter(&buf, tt.usage, tt.daily)
			out := buf.String()
			if tt.wantNothing {
				if out != "" {
					t.Fatalf("expected no footer, got:\n%s", out)
				}
				return
			}
			for _, want := range tt.wantParts {
				if !strings.Contains(out, want) {
					t.Errorf("footer missing %q:\n%s", want, out)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(out, absent) {
					t.Errorf("footer should not contain %q:\n%s", absent, out)
				}
			}
		})
	}
}

func TestFormatKTokens(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1.0k"},
		{143_200, "143.2k"},
	}
	for _, c := range cases {
		if got := formatKTokens(c.in); got != c.want {
			t.Errorf("formatKTokens(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
