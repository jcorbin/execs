package terminal

import (
	"os"

	"github.com/jcorbin/execs/internal/terminfo"
)

// Options creates a compound option from 0 or more options (returns nil in the
// 0 case).
func Options(opts ...Option) Option {
	if len(opts) == 0 {
		return nil
	}
	a := opts[0]
	opts = opts[1:]
	for len(opts) > 0 {
		b := opts[0]
		opts = opts[1:]
		if a == nil {
			a = b
			continue
		} else if b == nil {
			continue
		}
		as, haveAs := a.(options)
		bs, haveBs := b.(options)
		if haveAs && haveBs {
			a = append(as, bs...)
		} else if haveAs {
			a = append(as, b)
		} else if haveBs {
			a = append(options{a}, bs)
		} else {
			a = options{a, b}
		}
	}
	return a
}

// Option is an opaque option to pass to Open().
type Option interface {
	// init gets called while initializing internal terminal state; should not
	// manipulate external resources, but instead wire up a further lifecycle
	// option.
	init(term *Terminal) error
}

type optionFunc func(*Terminal) error

func (f optionFunc) init(term *Terminal) error { return f(term) }

type options []Option

func (os options) init(term *Terminal) error {
	for i := range os {
		if err := os[i].init(term); err != nil {
			return err
		}
	}
	return nil
}

// DefaultTerminfo loads default terminfo based on the TERM environment
// variable; basically it uses terminfo.Load(os.Getenv("TERM")).
var DefaultTerminfo = optionFunc(func(term *Terminal) error {
	if term.info != nil {
		return nil
	}
	info, err := terminfo.Load(os.Getenv("TERM"))
	if err == nil {
		term.info = info
	}
	return err
})

// Terminfo overrides any DefaultTerminfo with an explicit choice.
func Terminfo(info *terminfo.Terminfo) Option {
	return optionFunc(func(term *Terminal) error {
		term.info = info
		return nil
	})
}
