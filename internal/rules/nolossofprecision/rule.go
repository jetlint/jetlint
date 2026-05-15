// Package nolossofprecision implements the no-loss-of-precision rule:
// a numeric literal whose value cannot be represented exactly as a
// 64-bit float is silently rounded at parse time, so the running
// program sees a different value than the source. The rule flags
// literals whose printed form does not match the value JavaScript
// would actually store.
//
// Implementation tracks ESLint/oxlint's algorithm: for non-decimal
// literals we check whether the value fits in 53 bits; for decimal
// literals we normalize the source mantissa/exponent and compare to
// the stored f64 rendered via a `toPrecision`-style formatter.
package nolossofprecision

import (
	"math"
	"strconv"
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
)

const id = "no-loss-of-precision"

// New constructs a nolossofprecision rule instance.
func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindNumericLiteral: visit,
	}
}

func visit(ctx *engine.Context, n *wrapperchecker.Node) {
	// SourceText preserves the literal as written (with underscores,
	// leading zeros, hex/binary/octal prefix). LiteralText is the
	// parsed value, which has already lost precision for the cases
	// we want to flag.
	raw := n.SourceText()
	if raw == "" {
		return
	}
	if losesPrecision(raw) {
		ctx.Report(n, "This number literal will lose precision at runtime.")
	}
}

// losesPrecision reports whether the numeric literal `raw` cannot be
// stored exactly as an IEEE-754 double. Mirrors the upstream oxlint
// implementation.
func losesPrecision(raw string) bool {
	if raw == "" {
		return false
	}
	// TS-go's LiteralText for NumericLiteral may include suffixes
	// like `n` for BigInt — but BigInt nodes use KindBigIntLiteral,
	// so we don't expect to see them here. Be defensive anyway.
	if strings.HasSuffix(raw, "n") {
		return false
	}
	stripped := strings.TrimLeft(raw, "+-")
	if isNonBaseTen(stripped) {
		return notBaseTenLosesPrecision(stripped)
	}
	return baseTenLosesPrecision(raw)
}

func isNonBaseTen(s string) bool {
	if len(s) < 2 || s[0] != '0' {
		return false
	}
	switch s[1] {
	case 'b', 'B', 'o', 'O', 'x', 'X':
		return true
	}
	// Legacy octal: leading 0 followed only by digits 0-7. As soon as
	// we see a dot, 'e'/'E', or a digit 8/9, it's a decimal literal.
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			continue
		}
		if c < '0' || c > '7' {
			return false
		}
	}
	return len(s) > 1
}

// nonBaseTenLiteralIsExact mirrors oxlint's bit-counting check: walk
// each significand digit of a binary/octal/hex literal, accumulate the
// effective bit length, and return false as soon as it exceeds 53.
func nonBaseTenLiteralIsExact(raw string) bool {
	r := strings.TrimLeft(raw, "+-")
	var digits string
	var radix int
	switch {
	case strings.HasPrefix(r, "0b") || strings.HasPrefix(r, "0B"):
		digits = r[2:]
		radix = 2
	case strings.HasPrefix(r, "0o") || strings.HasPrefix(r, "0O"):
		digits = r[2:]
		radix = 8
	case strings.HasPrefix(r, "0x") || strings.HasPrefix(r, "0X"):
		digits = r[2:]
		radix = 16
	default:
		digits = strings.TrimLeft(r, "0")
		radix = 8
	}
	bitLen := 0
	for i := 0; i < len(digits); i++ {
		b := digits[i]
		if b == '_' {
			continue
		}
		var d byte
		switch {
		case b >= '0' && b <= '9':
			d = b - '0'
		case b >= 'a' && b <= 'f':
			d = b - 'a' + 10
		case b >= 'A' && b <= 'F':
			d = b - 'A' + 10
		default:
			return false
		}
		if bitLen == 0 {
			if d == 0 {
				continue
			}
			switch radix {
			case 2:
				bitLen = 1
			case 8:
				switch d {
				case 1:
					bitLen = 1
				case 2, 3:
					bitLen = 2
				default:
					bitLen = 3
				}
			default:
				switch {
				case d == 1:
					bitLen = 1
				case d <= 3:
					bitLen = 2
				case d <= 7:
					bitLen = 3
				default:
					bitLen = 4
				}
			}
		} else {
			switch radix {
			case 2:
				bitLen++
			case 8:
				bitLen += 3
			default:
				bitLen += 4
			}
		}
		if bitLen > 53 {
			return false
		}
	}
	return true
}

// notBaseTenLosesPrecision compares the source's digit string to a
// reconstructed digit string from the parsed f64. If they differ, the
// stored value lost information.
func notBaseTenLosesPrecision(stripped string) bool {
	if nonBaseTenLiteralIsExact(stripped) {
		return false
	}
	raw := strings.ToUpper(strings.ReplaceAll(stripped, "_", ""))
	value, ok := parseNonBaseTen(stripped)
	if !ok {
		// Couldn't parse — fall back to "lossy" so we don't silently
		// pass an unparseable literal.
		return true
	}
	var suffix string
	switch {
	case strings.HasPrefix(raw, "0B"):
		suffix = strconv.FormatUint(value, 2)
	case strings.HasPrefix(raw, "0X"):
		suffix = strconv.FormatUint(value, 16)
	default:
		suffix = strconv.FormatUint(value, 8)
	}
	return !strings.HasSuffix(raw, strings.ToUpper(suffix))
}

func parseNonBaseTen(raw string) (uint64, bool) {
	// Upstream casts node.value (already an f64) to u64. That cast is
	// lossy for values above 2**53, which is exactly the case we want
	// to detect: parse the digits as a big integer, convert to f64
	// (losing precision the same way the engine would), then take the
	// u64 of that f64.
	s := strings.ReplaceAll(raw, "_", "")
	negative := false
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	} else if strings.HasPrefix(s, "-") {
		negative = true
		s = s[1:]
	}
	var digits string
	var base int
	switch {
	case strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B"):
		digits = s[2:]
		base = 2
	case strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O"):
		digits = s[2:]
		base = 8
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		digits = s[2:]
		base = 16
	default:
		digits = strings.TrimLeft(s, "0")
		if digits == "" {
			return 0, true
		}
		base = 8
	}
	exact, err := strconv.ParseUint(digits, base, 64)
	var f float64
	if err == nil {
		f = float64(exact)
	} else {
		// Overflow uint64 — parse via big precision and cast through
		// f64. ParseFloat handles base-10 only, so we accumulate the
		// digits manually.
		f = 0
		for i := 0; i < len(digits); i++ {
			c := digits[i]
			var d byte
			switch {
			case c >= '0' && c <= '9':
				d = c - '0'
			case c >= 'a' && c <= 'f':
				d = c - 'a' + 10
			case c >= 'A' && c <= 'F':
				d = c - 'A' + 10
			default:
				return 0, false
			}
			f = f*float64(base) + float64(d)
		}
	}
	if math.IsInf(f, 0) {
		return 0, false
	}
	v := uint64(f)
	if negative {
		v = ^v + 1
	}
	return v, true
}

// baseTenLosesPrecision applies the upstream's two-stage check: the
// 15-significant-digit shortcut, then a normalize-and-compare against
// the stored f64.
func baseTenLosesPrecision(raw string) bool {
	value, err := strconv.ParseFloat(strings.ReplaceAll(raw, "_", ""), 64)
	if err != nil || math.IsInf(value, 0) {
		// Non-finite (e.g. `2e999`) loses precision — there's no
		// finite f64 that round-trips it.
		return true
	}
	if baseTenLiteralIsSafe(raw, value) {
		return false
	}
	stripped := strings.ReplaceAll(raw, "_", "")
	normRaw, ok := normalize(stripped, false)
	if !ok {
		return true
	}
	totalSig := len(normRaw.int) + len(normRaw.frac)
	if totalSig > 100 {
		return true
	}
	stored := toPrecision(value, totalSig)
	normStored, ok := normalize(stored, true)
	if !ok {
		return true
	}
	return !scientificEqual(normRaw, normStored)
}

const maxObviouslySafeSignificantDigits = 15

func baseTenLiteralIsSafe(raw string, value float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	r := strings.TrimLeft(raw, "+-")
	exponentStart := -1
	for i := 0; i < len(r); i++ {
		if r[i] == 'e' || r[i] == 'E' {
			exponentStart = i
			break
		}
	}
	if exponentStart != -1 {
		return false
	}
	mantissa := r
	digitIndex := 0
	firstNonZero := -1
	lastNonZero := -1
	fractionalHasNonZero := false
	dotDigitIndex := -1
	for i := 0; i < len(mantissa); i++ {
		b := mantissa[i]
		switch {
		case b == '_':
			continue
		case b == '.':
			dotDigitIndex = digitIndex
			continue
		case b >= '0' && b <= '9':
		default:
			return false
		}
		if b != '0' {
			if firstNonZero == -1 {
				firstNonZero = digitIndex
			}
			lastNonZero = digitIndex
			if dotDigitIndex != -1 {
				fractionalHasNonZero = true
			}
		}
		digitIndex++
	}
	if firstNonZero == -1 {
		return true
	}
	if value == 0.0 {
		return false
	}
	if dotDigitIndex != -1 && !fractionalHasNonZero {
		return dotDigitIndex-firstNonZero <= maxObviouslySafeSignificantDigits
	}
	lastSig := lastNonZero
	if dotDigitIndex != -1 {
		lastSig = digitIndex - 1
	}
	return lastSig-firstNonZero < maxObviouslySafeSignificantDigits
}

// scientificNotation represents a normalized number with a single
// non-zero integer digit, a fractional part, and an exponent. It
// matches the structure oxlint uses to compare the raw literal and the
// stored f64.
type scientificNotation struct {
	int  string
	frac string
	exp  int
}

func scientificEqual(a, b scientificNotation) bool {
	if a.int != b.int || a.frac != b.frac {
		return false
	}
	if a.int == "0" && a.frac == "" {
		return true
	}
	return a.exp == b.exp
}

// normalize converts a base-10 literal into scientificNotation, mirroring
// upstream's RawNum::new / normalize path.
func normalize(num string, parseAsFloat bool) (scientificNotation, bool) {
	coefficient := strings.TrimLeft(num, "+-")
	expStart := len(coefficient)
	for i := 0; i < len(coefficient); i++ {
		if coefficient[i] == 'e' || coefficient[i] == 'E' {
			expStart = i
			break
		}
	}
	hasDot := strings.Contains(coefficient[:expStart], ".")
	if hasDot {
		parseAsFloat = true
	}
	rn, ok := newRawNum(num)
	if !ok {
		return scientificNotation{}, false
	}
	return rn.normalize(parseAsFloat), true
}

type rawNum struct {
	intPart  string
	frac     string
	exp      int
}

func newRawNum(num string) (rawNum, bool) {
	s := strings.TrimLeft(num, "+-")
	// Skip leading zeros for integer part.
	withoutZeros := strings.TrimLeft(s, "0")
	intEnd := 0
	for intEnd < len(withoutZeros) && withoutZeros[intEnd] >= '0' && withoutZeros[intEnd] <= '9' {
		intEnd++
	}
	var intPart, withoutInt string
	if intEnd == 0 {
		intPart = "0"
	} else {
		intPart = withoutZeros[:intEnd]
	}
	withoutInt = withoutZeros[intEnd:]
	frac := ""
	withoutFrac := withoutInt
	if strings.HasPrefix(withoutInt, ".") {
		rest := withoutInt[1:]
		fracEnd := 0
		for fracEnd < len(rest) && rest[fracEnd] >= '0' && rest[fracEnd] <= '9' {
			fracEnd++
		}
		frac = rest[:fracEnd]
		withoutFrac = rest[fracEnd:]
	}
	exp := 0
	if strings.HasPrefix(withoutFrac, "e") || strings.HasPrefix(withoutFrac, "E") {
		expStr := withoutFrac[1:]
		v, err := strconv.Atoi(expStr)
		if err != nil {
			return rawNum{}, false
		}
		exp = v
	}
	return rawNum{intPart: intPart, frac: frac, exp: exp}, true
}

func (r rawNum) normalize(parseAsFloat bool) scientificNotation {
	if r.intPart == "0" && r.frac != "" {
		// Strip leading zeros in fractional.
		fracZeros := 0
		for fracZeros < len(r.frac) && r.frac[fracZeros] == '0' {
			fracZeros++
		}
		exp := r.exp - 1 - fracZeros
		frac := r.frac[fracZeros:]
		switch len(frac) {
		case 0:
			return scientificNotation{int: "0", frac: "", exp: exp}
		case 1:
			return scientificNotation{int: frac[:1], frac: "", exp: exp}
		default:
			return scientificNotation{int: frac[:1], frac: frac[1:], exp: exp}
		}
	}
	exp := r.exp + len(r.intPart) - 1
	if len(r.intPart) == 1 {
		return scientificNotation{int: r.intPart, frac: r.frac, exp: exp}
	}
	var frac string
	if r.frac == "" {
		if parseAsFloat {
			frac = r.intPart[1:]
		} else {
			trimmed := strings.TrimRight(r.intPart, "0")
			if len(trimmed) <= 1 {
				frac = ""
			} else {
				frac = trimmed[1:]
			}
		}
	} else {
		frac = r.intPart[1:] + r.frac
	}
	return scientificNotation{int: r.intPart[:1], frac: frac, exp: exp}
}

// toPrecision reproduces JS's Number.prototype.toPrecision(num, precision)
// well enough to compare against the source literal. It mirrors oxlint's
// implementation: format the value with enough fractional digits, locate
// the first non-zero digit, round to `precision` digits, then place the
// decimal point or convert to scientific notation.
func toPrecision(num float64, precision int) string {
	if precision < 1 {
		precision = 1
	}
	if precision > 100 {
		precision = 100
	}
	if math.IsNaN(num) {
		return "NaN"
	}
	if math.IsInf(num, 1) {
		return "Infinity"
	}
	if math.IsInf(num, -1) {
		return "-Infinity"
	}
	prefix := ""
	if num < 0 {
		prefix = "-"
		num = -num
	}
	var suffix string
	var exponent int
	if num == 0 {
		suffix = strings.Repeat("0", precision)
		exponent = 0
	} else {
		fractionalDigits := fractionalDigitsForPrecision(num, precision)
		suffix = strconv.FormatFloat(num, 'f', fractionalDigits, 64)
		exponent = fltStrToExp(suffix)
		if exponent < 0 {
			suffix = suffix[1-exponent:]
		} else if dot := strings.Index(suffix, "."); dot >= 0 {
			suffix = suffix[:dot] + suffix[dot+1:]
		}
		rounded, carry := roundToPrecision(suffix, precision)
		suffix = rounded
		if carry {
			exponent++
		}
		greatExp := exponent >= precision
		if exponent < -6 || greatExp {
			out := suffix
			if precision > 1 {
				out = out[:1] + "." + out[1:]
			}
			out += "e"
			if greatExp {
				out += "+"
			}
			out += strconv.Itoa(exponent)
			return prefix + out
		}
	}
	eInc := exponent + 1
	if eInc == precision {
		return prefix + suffix
	}
	if exponent >= 0 {
		suffix = suffix[:eInc] + "." + suffix[eInc:]
	} else {
		prefix += "0."
		prefix += strings.Repeat("0", -eInc)
	}
	return prefix + suffix
}

func fltStrToExp(flt string) int {
	nonZeroEncountered := false
	dotEncountered := false
	for i := 0; i < len(flt); i++ {
		c := flt[i]
		if c == '.' {
			if nonZeroEncountered {
				return i - 1
			}
			dotEncountered = true
		} else if c != '0' {
			if dotEncountered {
				return 1 - i
			}
			nonZeroEncountered = true
		}
	}
	return len(flt) - 1
}

func roundToPrecision(digits string, precision int) (string, bool) {
	if len(digits) > precision {
		toRound := digits[precision:]
		digits = digits[:precision]
		last := digits[len(digits)-1]
		digits = digits[:len(digits)-1]
		if len(toRound) > 0 && toRound[0] > '4' {
			last++
		}
		if last == ':' { // '9' + 1
			replacement := []byte{'0'}
			propagated := false
			for i := len(digits) - 1; i >= 0; i-- {
				c := digits[i]
				var d byte
				switch {
				case c >= '0' && c <= '8' && !propagated:
					d = c + 1
					propagated = true
				case !propagated:
					d = '0'
				default:
					d = c
				}
				replacement = append(replacement, d)
				if d != '0' {
					propagated = true
				}
			}
			var b strings.Builder
			carry := !propagated
			if !propagated {
				b.WriteByte('1')
				// drop the trailing '0' we seeded
				replacement = replacement[:len(replacement)-1]
			}
			for i := len(replacement) - 1; i >= 0; i-- {
				b.WriteByte(replacement[i])
			}
			return b.String(), carry
		}
		return string(append([]byte(digits), last)), false
	}
	return digits + strings.Repeat("0", precision-len(digits)), false
}

func fractionalDigitsForPrecision(num float64, precision int) int {
	formatted := strconv.FormatFloat(num, 'e', -1, 64)
	exp := 0
	if idx := strings.LastIndex(formatted, "e"); idx >= 0 {
		v, err := strconv.Atoi(formatted[idx+1:])
		if err == nil {
			exp = v
		}
	}
	if exp < 0 {
		v := -exp
		v += precision + 1
		if v < 100 {
			v = 100
		}
		return v
	}
	return 100
}
