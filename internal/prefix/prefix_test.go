package prefix

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"24ab", "24ab"},
		{"0x24AB", "24ab"},
		{"0X2", "2"},
		{"  abc ", "abc"},
	}
	for _, tt := range tests {
		p, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if p.String() != tt.want || p.Len() != len(tt.want) {
			t.Errorf("Parse(%q) = %q, want %q", tt.in, p, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "0x", "xyz", "12g4", "0x-1", strings.Repeat("a", MaxDigits+1)} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q): want error", in)
		}
	}
	if _, err := Parse(strings.Repeat("a", MaxDigits)); err != nil {
		t.Errorf("Parse(128 digits): %v", err)
	}
}

func TestMatches(t *testing.T) {
	digest := []byte{0x24, 0xab, 0xcd, 0xef}
	tests := []struct {
		prefix string
		want   bool
	}{
		{"2", true},
		{"24", true},
		{"24a", true},
		{"24ab", true},
		{"24abc", true},
		{"24abcdef", true},
		{"3", false},
		{"25", false},
		{"24b", false},
		{"24abce", false},
		{"24abcdef0", false}, // longer than the digest
	}
	for _, tt := range tests {
		p, err := Parse(tt.prefix)
		if err != nil {
			t.Fatal(err)
		}
		if got := p.Matches(digest); got != tt.want {
			t.Errorf("%q.Matches(%x) = %v, want %v", tt.prefix, digest, got, tt.want)
		}
	}
}
