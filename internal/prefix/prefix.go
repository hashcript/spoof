// Package prefix parses and matches the target hexstring (R-05, R-07).
package prefix

import (
	"errors"
	"fmt"
	"strings"
)

// MaxDigits is the number of hex digits in a SHA-512 digest; no prefix can be longer.
const MaxDigits = 128

// Prefix is a normalised target prefix: lowercase hex digits without "0x".
// An odd number of digits is matched nibble by nibble.
type Prefix struct {
	digits string
	full   []byte // whole bytes to match
	half   int    // high nibble of the next byte to match, or -1
}

// Parse normalises s (optional 0x, any case) and validates it.
func Parse(s string) (Prefix, error) {
	d := strings.ToLower(strings.TrimSpace(s))
	d = strings.TrimPrefix(d, "0x")
	if d == "" {
		return Prefix{}, errors.New("hexstring is empty")
	}
	if len(d) > MaxDigits {
		return Prefix{}, fmt.Errorf("hexstring has %d digits; a SHA-512 digest has only %d", len(d), MaxDigits)
	}
	p := Prefix{digits: d, half: -1}
	for i := 0; i < len(d); i++ {
		v, ok := nibble(d[i])
		if !ok {
			return Prefix{}, fmt.Errorf("hexstring %q contains %q, which is not a hex digit", s, d[i:i+1])
		}
		if i%2 == 0 {
			if i == len(d)-1 {
				p.half = v
			} else {
				p.full = append(p.full, byte(v<<4))
			}
		} else {
			p.full[len(p.full)-1] |= byte(v)
		}
	}
	return p, nil
}

func nibble(c byte) (int, bool) {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0'), true
	case 'a' <= c && c <= 'f':
		return int(c-'a') + 10, true
	}
	return 0, false
}

// String returns the normalised digits.
func (p Prefix) String() string { return p.digits }

// Len returns the number of hex digits (nibbles).
func (p Prefix) Len() int { return len(p.digits) }

// Matches reports whether digest starts with the prefix.
func (p Prefix) Matches(digest []byte) bool {
	if len(digest)*2 < len(p.digits) {
		return false
	}
	for i, b := range p.full {
		if digest[i] != b {
			return false
		}
	}
	return p.half < 0 || int(digest[len(p.full)]>>4) == p.half
}
