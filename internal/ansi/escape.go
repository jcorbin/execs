package ansi

import "fmt"

// Escape is an ANSI escape or control function mapped into the unicode world:
// - U+0000-U+001F: C0 controls
// - U+0080-U+009F: C1 controls
// - U+EF20-U+EF2F: character set selection functions
// - U+EF30-U+EF3F: private ESCape-sequence functions
// - U+EF40-U+EF5F: non-standard ESCape-sequence functions
// - U+EF60-U+EF7E: standard ESCape-sequence functions
// -        U+EF7F: malformed ESC sequence
// - U+EFC0-U+EFFE: CSI functions
// -        U+EFFF: malformed CSI sequence
type Escape rune

// ESC constructs an ESCape sequence indicator named by the given byte.
func ESC(b byte) Escape { return Escape(0xEF00 | 0x7F&rune(b)) }

// CSI constructs a CSI control sequence indicator named byt he given byte.
func CSI(b byte) Escape { return Escape(0xEF80 | rune(b)) }

// ESC returns the byte name of the represented ESCape sequence if any; returns
// true only if so.
func (e Escape) ESC() (byte, bool) {
	if 0xEF00 < e && e < 0xEF7F {
		return byte(e & 0x7F), true
	}
	return 0, false
}

// CSI returns the byte name of the represented CSI control sequence if any;
// returns true only if so.
func (e Escape) CSI() (byte, bool) {
	if 0xEF80 < e && e < 0xEFFF {
		return byte(e & 0x7F), true
	}
	return 0, false
}

// C1Names provides representation names for the C1 extended-ASCII control
// block.
var C1Names = []string{
	"<RES@>",
	"<RESA>",
	"<RESB>",
	"<RESC>",
	"<IND>",
	"<NEL>",
	"<SSA>",
	"<ESA>",
	"<HTS>",
	"<HTJ>",
	"<VTS>",
	"<PLD>",
	"<PLU>",
	"<RI>",
	"<SS2>",
	"<SS3>",
	"<DCS>",
	"<PU1>",
	"<PU2>",
	"<STS>",
	"<CCH>",
	"<MW>",
	"<SPA>",
	"<EPA>",
	"<RESX>",
	"<RESY>",
	"<RESZ>",
	"<CSI>",
	"<ST>",
	"<OSC>",
	"<PM>",
	"<APC>",
}

func (e Escape) String() string {
	switch {
	case e <= 0x1F:
		return "^" + string(byte(0x40^e))
	case e == 0x7F:
		return "^?"
	case 0x80 <= e && e <= 0x9F:
		return C1Names[e&^0x80]
	case 0xEF30 <= e && e <= 0xEF7E:
		return fmt.Sprintf("ESC+%s", string(byte(e)))
	case 0xEFC0 <= e && e <= 0xEFFE:
		return fmt.Sprintf("CSI+%s", string(byte(0x7f&e)))
	case e == 0xEF7F:
		return "ESC+INVALID"
	case e == 0xEFFF:
		return "CSI+INVALID"
	default:
		return fmt.Sprintf("U+%04X", rune(e))
	}
}
