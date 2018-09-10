package main

import (
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/ecs"
)

type descSpec struct {
	typ   ecs.Type
	label string
	desc  func(ecs.Entity) fmt.Stringer
}

func describe(w io.Writer, ent ecs.Entity, spec []descSpec) {
	describer{
		width: 15,
		spec:  spec,
	}.describe(w, ent)
}

type describer struct {
	width int
	spec  []descSpec
}

func (desc describer) describe(w io.Writer, ent ecs.Entity) {
	valWidth := 4
	for _, sp := range desc.spec {
		if c := utf8.RuneCountInString(sp.label); valWidth < c {
			valWidth = c
		}
	}

	typ := ent.Type()
	// fmt.Fprintf(w, "Scope: % *v", desc.width, ent.Scope)
	fmt.Fprintf(w, "% *s: % *v", valWidth, "ID", desc.width, ent.ID)
	fmt.Fprintf(w, "\r\n% *s: % *v", valWidth, "Type", desc.width, typ)
	for _, sp := range desc.spec {
		if typ&sp.typ == sp.typ {
			if sp.desc != nil {
				fmt.Fprintf(w, "\r\n% *s: % *v", valWidth, sp.label, desc.width, sp.desc(ent))
			} else {
				fmt.Fprintf(w, "\r\n% *s", valWidth, sp.label)
			}
		}
	}
}
