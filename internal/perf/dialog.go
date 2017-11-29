package perf

import (
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
)

// dialog is a modal dialog with an action bar.
//
// TODO: break out; also separate the modality concern, finally start having an
// explicit ui input priocity stack which you could shift this onto.
type dialog struct {
	// bar actionBar
}

func (dia dialog) RenderSize() (wanted, needed point.Point) {
	return needed, needed
}

func (dia dialog) Render(view.Grid) {
}
