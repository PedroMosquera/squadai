package pricing

import (
	"encoding/json"
	"testing"

	"github.com/PedroMosquera/squadai/internal/modelcatalog"
)

// pricing.json still carries a per-model price list that duplicates the
// catalog. Cost reports read the catalog, so until one copy is removed a
// drifted duplicate silently misleads whoever edits it.
func TestPricingJSONAgreesWithCatalog(t *testing.T) {
	var file struct {
		Models []struct {
			Prefix string  `json:"prefix"`
			Input  float64 `json:"input_per_million"`
			Output float64 `json:"output_per_million"`
		} `json:"models"`
	}
	if err := json.Unmarshal(pricingJSON, &file); err != nil {
		t.Fatalf("parse pricing.json: %v", err)
	}
	cat, err := modelcatalog.Load(t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("load embedded catalog: %v", err)
	}

	prefixes := make(map[string]bool, len(file.Models))
	for _, row := range file.Models {
		prefixes[row.Prefix] = true
		p, ok := cat.Pricing(row.Prefix)
		if !ok {
			t.Errorf("pricing.json row %q is unknown to the model catalog", row.Prefix)
			continue
		}
		if p.InputPerMTok != row.Input || p.OutputPerMTok != row.Output {
			t.Errorf("price drift for %q: pricing.json $%v/$%v, catalog $%v/$%v",
				row.Prefix, row.Input, row.Output, p.InputPerMTok, p.OutputPerMTok)
		}
	}
	for _, id := range cat.ModelIDs() {
		if !prefixes[id] {
			t.Errorf("catalog model %q has no pricing.json row", id)
		}
	}
}
