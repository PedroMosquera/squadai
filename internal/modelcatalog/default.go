package modelcatalog

import (
	"os"
	"sync"
)

var (
	defaultMu  sync.Mutex
	defaultCat *Catalog
)

// Default returns the process-wide catalog, loading it once on first use
// from the user home directory and current working directory. Load failures
// (e.g. a corrupt override file) degrade to the embedded catalog so that
// low-level consumers (pricing, tokenizer) never fail.
func Default() *Catalog {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultCat != nil {
		return defaultCat
	}
	homeDir, _ := os.UserHomeDir()
	projectDir, _ := os.Getwd()
	c, err := Load(homeDir, projectDir)
	if err != nil {
		c = FromFile(loadEmbedded(), SourceEmbedded)
	}
	defaultCat = c
	return defaultCat
}

// SetDefaultForTest replaces the process-wide catalog and returns a restore
// function. Passing nil resets to lazy re-load on next Default() call.
func SetDefaultForTest(c *Catalog) (restore func()) {
	defaultMu.Lock()
	prev := defaultCat
	defaultCat = c
	defaultMu.Unlock()
	return func() {
		defaultMu.Lock()
		defaultCat = prev
		defaultMu.Unlock()
	}
}
