package doc

import "strings"

// Order keys are fractional: a key between any two keys always exists, so
// inserting a node writes one row. Keys use the digits 0-9A-Za-z, whose
// byte order is their numeric order.
const ordDigits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// ordLimit bounds a key's length; past it a parent's children are re-keyed.
const ordLimit = 24

func ordDigit(c byte) int { return strings.IndexByte(ordDigits, c) }

// ordBetween returns a key strictly between a and b, where "" stands for
// the lowest bound when it is a and the highest when it is b.
func ordBetween(a, b string) string {
	var out []byte
	for i := 0; ; i++ {
		lo := 0
		if i < len(a) {
			lo = ordDigit(a[i])
		}
		hi := len(ordDigits)
		if b != "" && i < len(b) {
			hi = ordDigit(b[i])
		}
		if hi-lo > 1 {
			return string(append(out, ordDigits[(lo+hi)/2]))
		}
		out = append(out, ordDigits[lo])
		if hi-lo == 1 {
			// The next digit only has to clear a's tail.
			b = ""
		}
	}
}

// ordSpread returns n evenly spaced keys of the shortest length that fits.
func ordSpread(n int) []string {
	width, room := 1, len(ordDigits)
	for room <= n {
		width++
		room *= len(ordDigits)
	}
	keys := make([]string, n)
	step := room / (n + 1)
	for i := range keys {
		v := step * (i + 1)
		key := make([]byte, width)
		for j := width - 1; j >= 0; j-- {
			key[j] = ordDigits[v%len(ordDigits)]
			v /= len(ordDigits)
		}
		keys[i] = string(key)
	}
	return keys
}
