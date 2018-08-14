package ansi

import (
	"bytes"
	"strconv"
)

// CTLSeq is a convenience constructor for a CSI-led control sequence with the
// given name and argument bytes.
func CTLSeq(name byte, arg []byte) ControlSeq { return ControlSeq{name: name, argBytes: arg} }

// CTLSeqString is a convenience constructor for a CSI-led control sequence
// with the given name and argument string.
func CTLSeqString(name byte, arg string) ControlSeq { return ControlSeq{name: name, argString: arg} }

// ControlSeq represents a CSI-led control sequence for writing to some output.
type ControlSeq struct {
	name         byte
	argBytes     []byte
	argString    string
	argN         int
	argNums      [4]int
	argExtraNums []int
}

// With adds integer arguments to a copy of the control sequence, and returns
// the copy.
func (cs ControlSeq) With(args ...int) ControlSeq {
	if extraNeed := len(args) - 4 + cs.argN; extraNeed > 4 {
		if cs.argExtraNums == nil {
			cs.argExtraNums = make([]int, 0, extraNeed)
		} else {
			cs.argExtraNums = append(
				make([]int, 0, len(cs.argExtraNums)+extraNeed),
				cs.argExtraNums...)
		}
	}
	for ; len(args) > 0; args = args[1:] {
		if cs.argN < 4 {
			cs.argNums[cs.argN] = args[0]
			cs.argN++
		} else {
			cs.argExtraNums = append(cs.argExtraNums, args[0])
		}
	}
	return cs
}

// ControlSeqs represents a series of CSI-led control sequences.
type ControlSeqs []ControlSeq

func writeInt(buf *bytes.Buffer, n int) {
	var tmp [64 + 1]byte
	_, _ = buf.Write(strconv.AppendInt(tmp[:], int64(n), 10))
}

// WriteIntoBuffer writes the control sequence into the given byte buffer.
func (cs ControlSeq) WriteIntoBuffer(buf *bytes.Buffer) {
	_, _ = buf.WriteString("\x1b[")
	if len(cs.argBytes) > 0 {
		_, _ = buf.Write(cs.argBytes)
	}
	if cs.argString != "" {
		_, _ = buf.WriteString(cs.argString)
	}
	j := 0
	for i := 0; i < cs.argN; i++ {
		if j > 0 {
			_ = buf.WriteByte(';')
		}
		writeInt(buf, cs.argNums[i])
		j++
	}
	for i := range cs.argExtraNums {
		if j > 0 {
			_ = buf.WriteByte(';')
		}
		writeInt(buf, cs.argExtraNums[i])
		j++
	}
	_ = buf.WriteByte(cs.name)
}

// WriteIntoBuffer writes the control sequences into the given byte buffer.
func (css ControlSeqs) WriteIntoBuffer(buf *bytes.Buffer) {
	for i := range css {
		css[i].WriteIntoBuffer(buf)
	}
}

func (cs ControlSeq) String() string {
	var buf bytes.Buffer
	buf.Grow(32)
	cs.WriteIntoBuffer(&buf)
	return buf.String()
}

func (css ControlSeqs) String() string {
	var buf bytes.Buffer
	buf.Grow(len(css) * 32)
	for i := range css {
		css[i].WriteIntoBuffer(&buf)
	}
	return buf.String()
}
