package view

import "github.com/jcorbin/execs/internal/point"

// Stepable represents something that can run bound to a View; see View.Run.
type Stepable interface {
	// TODO: maybe decompose finer grained, e.g. a step method
	Step() bool
}

// Errable may be implemented by a Stepable to afford errors after stepping
// stops.
type Errable interface {
	Stepable
	Err() error
}

// Renderable is implemented by a Stepable to render the view prior to each
// step.
type Renderable interface {
	Stepable
	Render(*Context)
}

// JustKeepRunning starts a view, and then running newly minted Runables
// provided by the given factory until an error occurs, or the user quits.
// Useful for implementing main.main.
func JustKeepRunning(factory func(v *View) (Stepable, error)) error {
	var v View
	if err := v.Start(); err != nil {
		return err
	}
	defer v.Stop()
	for v.err == nil && v.running {
		s, err := factory(&v)
		if err != nil {
			v.err = err
			break
		}
		v.stepit(s)
	}
	return v.err
}

// Run a Stepable under this view. The Stepable may implement Errable, and if
// so any error it indicates after steps are done is retained.  Returns false
// if any error (step or view caused) occurred.
func (v *View) Run(s Stepable) bool {
	if v.err != nil {
		return false
	}
	v.stepit(s)
	if v.err != nil {
		v.Stop()
	}
	return v.err == nil && v.running
}

func (v *View) stepit(s Stepable) {
	ra, _ := s.(Renderable)
	for {
		if ra != nil {
			v.renderLock.Lock()
			if v.size.X <= 0 || v.size.Y <= 0 {
				v.size = termboxSize()
			}
			v.ctx.Avail = v.size.Sub(point.Point{Y: len(v.ctx.Footer) + len(v.ctx.Header)})
			ra.Render(&v.ctx)
			err := v.render()
			v.renderLock.Unlock()
			if err != nil {
				v.err = err
				return
			}
		}
		if !s.Step() {
			break
		}
		// TODO: observability / introspection / other Nice To Haves?
		// TODO: compulsorily implement done check?
	}
	if ea, ok := s.(Errable); ok && v.err == nil {
		v.err = ea.Err()
	}
}
