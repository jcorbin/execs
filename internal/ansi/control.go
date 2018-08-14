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

// IsCharacterSetControl returns true if the given rune is used for selecting
// classic ANSI terminal character set; such controls can be ignored in a
// modern UTF-8 terminal.
func IsCharacterSetControl(r rune) bool {
	switch r {
	case
		0x000E, // SO     Shift Out, switch to G1 (other half of character set)
		0x000F, // SI     Shift In, switch to G0 (normal half of character set)
		0x008E, // SS2    Single Shift to G2
		0x008F, // SS3    Single Shift to G3 (VT100 uses this for sending PF keys)
		0xEF28, // ESC+(  SCS - Select G0 character set (choice of 63 standard, 16 private)
		0xEF29, // ESC+)  SCS - Select G1 character set (choice of 63 standard, 16 private)
		0xEF2A, // ESC+*  SCS - Select G2 character set
		0xEF2B, // ESC++  SCS - Select G3 character set
		0xEF2C, // ESC+,  SCS - Select G0 character set (additional 63+16 sets)
		0xEF2D, // ESC+-  SCS - Select G1 character set (additional 63+16 sets)
		0xEF2E, // ESC+.  SCS - Select G2 character set
		0xEF2F, // ESC+/  SCS - Select G3 character set
		0xEF6B, // ESC+k  NAPLPS lock-shift G1 to GR
		0xEF6C, // ESC+l  NAPLPS lock-shift G2 to GR
		0xEF6D, // ESC+m  NAPLPS lock-shift G3 to GR
		0xEF6E, // ESC+n  LS2 - Shift G2 to GL (extension of SI) VT240,NAPLPS
		0xEF6F, // ESC+o  LS3 - Shift G3 to GL (extension of SO) VT240,NAPLPS
		0xEF7C, // ESC+|  LS3R - VT240 lock-shift G3 to GR
		0xEF7D, // ESC+}  LS2R - VT240 lock-shift G2 to GR
		0xEF7E: // ESC+~  LS1R - VT240 lock-shift G1 to GR
		return true
	}
	return false
}
