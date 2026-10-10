package scan

import (
	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/codex"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/pi"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// AllAdapters returns every built-in adapter, detected locally or not. A repo
// carries config for harnesses its CI runner never has installed, so scan
// looks at all of them rather than at the detected set apply uses.
func AllAdapters() []domain.Adapter {
	return []domain.Adapter{
		opencode.New(),
		claude.New(),
		vscode.New(),
		cursor.New(),
		windsurf.New(),
		pi.New(),
		codex.New(),
	}
}
