package perf

import (
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
)

// perfDetail is a dialog for viewing last-round perf data.
type perfDetail struct {
	dialog
	*Perf
}

func (pdet perfDetail) HandleKey(view.KeyEvent) bool {
	return false
}

func (pdet perfDetail) RenderSize() (wanted, needed point.Point) {
	return needed, needed
}

func (pdet perfDetail) Render(view.Grid) {
}
