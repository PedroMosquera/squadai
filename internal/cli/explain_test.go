package cli

import (
	"strings"
	"testing"
)

func TestExplainTopic_BudgetSummaryModeRendersCondensedContent(t *testing.T) {
	text, ok := explainTopic("budget")
	if !ok {
		t.Fatal("budget topic missing")
	}
	if strings.Contains(text, "skips") {
		t.Errorf("summary mode no longer skips writing; text still says so:\n%s", text)
	}
	for _, want := range []string{"memory stub", "standards/summary.md", "orchestrator digest"} {
		if !strings.Contains(text, want) {
			t.Errorf("budget topic should name the condensed variant %q", want)
		}
	}
}

func TestExplainTopic_ErrorCodesListsBudgetExceeded(t *testing.T) {
	text, ok := explainTopic("error-codes")
	if !ok {
		t.Fatal("error-codes topic missing")
	}
	if !strings.Contains(text, "E-901") || !strings.Contains(text, "--against-budget") {
		t.Errorf("error-codes topic should document E-901 for --against-budget:\n%s", text)
	}
}
