package view

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/point"
)

// Context contains a convenience Header, Footer, and list of Log-ed messages
// that can be rendered by a Client.
type Context struct {
	Header []string
	Footer []string
	Logs   []string
	Grid   Grid
}

// Render the context into the given terminal grid.
func (ctx Context) Render(termGrid Grid) error {
	header := layoutLines(ctx.Header, termGrid.Size)
	footer := layoutLines(ctx.Footer, termGrid.Size)
	// TODO: prior footer default was right-aligned; restore that once
	// naturalized to Grid; also restore the bottom-aligned footer property to
	// both right/left side independently

	if len(ctx.Logs) > 0 {
		header = append(header[:len(header):len(header)], ctx.Logs...)
	}
	termGrid.Copy(ctx.Grid)
	for i := 0; i < len(header); i++ {
		termGrid.WriteString(i, AlignLeft, header[i])
	}
	for i, j := len(footer)-1, 1; i >= 0; i, j = i-1, j+1 {
		termGrid.WriteString(termGrid.Size.Y-j, AlignLeft, footer[i])
	}
	return nil
}

// Log adds a line to the internal log buffer. As much tail of the log buffer
// is displayed after the header as possible; at least 5 lines.
func (ctx *Context) Log(mess string, args ...interface{}) {
	ctx.Logs = append(ctx.Logs, fmt.Sprintf(mess, args...))
}

// ClearLog clears the internal log buffer.
func (ctx *Context) ClearLog() { ctx.Logs = ctx.Logs[:0] }

// SetHeader copies the given lines into the internal header buffer, replacing
// any prior.
func (ctx *Context) SetHeader(lines ...string) {
	if cap(ctx.Header) < len(lines) {
		ctx.Header = make([]string, len(lines))
	} else {
		ctx.Header = ctx.Header[:len(lines)]
	}
	copy(ctx.Header, lines)
}

// SetFooter copies the given lines into the internal footer buffer, replacing
// any prior.
func (ctx *Context) SetFooter(lines ...string) {
	if cap(ctx.Footer) < len(lines) {
		ctx.Footer = make([]string, len(lines))
	} else {
		ctx.Footer = ctx.Footer[:len(lines)]
	}
	copy(ctx.Footer, lines)
}

func layoutLines(parts []string, size point.Point) []string {
	// TODO: first-pass, reify this into some sort of `type lineLayer struct`
	// before adding centering
	// TODO: naturalize this onto Grid for less copies

	lparts := layoutParts(parts, layoutAlignLeft, size)
	rparts := layoutParts(parts, layoutAlignRight, size)

	// combine left and right parts
	n := len(lparts)
	if cap(rparts) > n {
		n = len(rparts)
	}
	lines := make([]string, 0, n)
	i := 0
	for ; i < len(lparts) && i < len(rparts); i++ {
		rem := size.X
		part := lparts[i]
		rem -= len(part)
		if gap := rem - len(rparts[i]); gap > 0 {
			part += strings.Repeat(" ", gap)
			part += rparts[i]
		} else if gap < 0 {
			part += rparts[i][:len(rparts[i])+gap]
		} else {
			part += rparts[i]
		}
		lines = append(lines, part)
	}
	for ; i < len(lparts); i++ {
		lines = append(lines, lparts[i])
	}
	for ; i < len(rparts); i++ {
		lines = append(lines,
			strings.Repeat(" ", size.X-len(rparts[i]))+rparts[i])
	}

	// XXX
	for j := range parts {
		part, opts := scanLayoutOpts(parts[j])
		if opts&(layoutAlignLeft|layoutAlignRight) == 0 {
			lines = append(lines, fmt.Sprintf("[%d] %04x %q", j, opts, part))
			i++
		}
	}

	return lines[:i]
}

func layoutParts(in []string, mask layoutOption, size point.Point) []string {
	out := make([]string, 1, len(in))
	for i := range in {
		part, opts := scanLayoutOpts(in[i])
		if opts&mask == 0 {
			continue
		}
		if opts&layoutClear != 0 {
			out = append(out, "")
		}
		prior := len(out[len(out)-1])
		rem := size.X - prior - len(part)
		if prior > 0 {
			rem--
		}
		if rem < 0 && prior > 0 {
			rem += prior
			prior, out = 0, append(out, "")
		}
		if rem < 0 {
			part = part[:len(part)+rem]
		}
		if j := len(out) - 1; prior > 0 {
			out[j] = out[j] + " " + part
		} else {
			out[j] = part
		}
	}
	return out
}

type layoutOption uint16

const (
	layoutClear layoutOption = 1 << iota
	layoutAlignLeft
	layoutAlignRight
	// layoutAlignCenter // TODO
)

func scanLayoutOpts(s string) (rest string, opts layoutOption) {
	for len(s) > 0 {
		switch r, n := utf8.DecodeRuneInString(s); r {
		case '.':
			opts |= layoutClear
			s = s[n:]
			continue
		case '<':
			opts |= layoutAlignLeft
			s = s[n:]
		case '>':
			opts |= layoutAlignRight
			s = s[n:]
		}
		break
	}
	if opts == 0 {
		opts = layoutAlignLeft | layoutClear
	}
	if opts&(layoutAlignLeft|layoutAlignRight) == 0 {
		opts |= layoutAlignLeft
	}
	return s, opts
}
