package terminal

import (
	copsDisp "github.com/jcorbin/execs/internal/cops/display"
)

// TODO fully steal or revert to properly borrowing cops's cursor

var StartCursor = copsDisp.Start

type Cursor = copsDisp.Cursor
