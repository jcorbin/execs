package main

import "github.com/jcorbin/execs/internal/imtui"

type imid int

type ui struct {
	imtui.Core

	next   imid
	focus  imid
	active imid
}

func (ui *ui) nextID() imid {
	id := ui.next
	ui.next++
	return id
}

type client interface {
	draw(*ui) error
}

type clientFunc func(*ui) error

func (f clientFunc) draw(ui *ui) error { return f(ui) }

func (ui *ui) run(client client) error {
	return imtui.Run(&ui.Core, imtui.DrawFunc(func(_ *imtui.Core) error {
		ui.next = 1
		return client.draw(ui)
	}))
}

func runUI(client client) error {
	var ui ui
	return ui.run(client)
}
