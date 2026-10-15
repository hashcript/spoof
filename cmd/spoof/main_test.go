package main

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashcript/spoof/internal/testimg"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEndToEnd(t *testing.T) {
	inputs := map[string][]byte{"in.png": testimg.PNG(t, 20, 10), "in.jpg": testimg.JPEG(t, 20, 10)}
	for name, src := range inputs {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			in := filepath.Join(dir, name)
			out := filepath.Join(dir, "out"+filepath.Ext(name))
			writeFile(t, in, src)

			code, stdout, stderr := runCLI(t, "0x24AB", in, out)
			if code != exitOK {
				t.Fatalf("exit %d, stderr: %s", code, stderr)
			}
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			d := sha512.Sum512(data)
			if got := strings.TrimSpace(stdout); got != hex.EncodeToString(d[:]) {
				t.Errorf("stdout %q is not the output digest %x", got, d)
			}
			if !strings.HasPrefix(stdout, "24ab") {
				t.Errorf("digest %q does not start with 24ab", stdout)
			}
			if got, _ := os.ReadFile(in); !bytes.Equal(got, src) {
				t.Error("input file was modified")
			}

			// Running again on the output with --force replaces the spoof block.
			again := filepath.Join(dir, "again"+filepath.Ext(name))
			code, stdout, stderr = runCLI(t, "--workers", "1", out, again, "f0", "--force")
			if code != exitUsage {
				t.Fatalf("misordered args: exit %d, stderr: %s", code, stderr)
			}
			code, stdout, stderr = runCLI(t, "--workers=1", "f0", out, again)
			if code != exitOK || !strings.HasPrefix(stdout, "f0") {
				t.Fatalf("exit %d, stdout %q, stderr: %s", code, stdout, stderr)
			}
			second, _ := os.ReadFile(again)
			if len(second) != len(data) {
				t.Errorf("re-run changed size from %d to %d; spoof blocks stacked", len(data), len(second))
			}
		})
	}
}

func TestFlagsAfterPositionals(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.png")
	writeFile(t, in, testimg.PNG(t, 4, 4))
	out := filepath.Join(dir, "out.png")
	if code, _, stderr := runCLI(t, "a", in, out, "--workers", "2", "--timeout=1m"); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, stderr)
	}
}

func TestErrors(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.png")
	writeFile(t, in, testimg.PNG(t, 4, 4))
	gif := filepath.Join(dir, "in.gif")
	writeFile(t, gif, []byte("GIF89a\x01\x00\x01\x00"))
	broken := filepath.Join(dir, "broken.png")
	writeFile(t, broken, testimg.PNG(t, 4, 4)[:40])
	existing := filepath.Join(dir, "existing.png")
	writeFile(t, existing, []byte("keep"))
	out := filepath.Join(dir, "out.png")

	tests := []struct {
		name string
		args []string
		code int
		msg  string
	}{
		{"no args", nil, exitUsage, "usage"},
		{"too few args", []string{"ab", in}, exitUsage, "usage"},
		{"bad hex", []string{"xyz", in, out}, exitUsage, "not a hex digit"},
		{"empty hex", []string{"0x", in, out}, exitUsage, "empty"},
		{"too long", []string{strings.Repeat("0", 13), in, out}, exitUsage, "--max-digits"},
		{"bad max-digits", []string{"--max-digits", "200", "ab", in, out}, exitUsage, "--max-digits"},
		{"unknown flag", []string{"--nope", "ab", in, out}, exitUsage, "flag provided but not defined"},
		{"same file", []string{"ab", in, in}, exitUsage, "same file"},
		{"output exists", []string{"ab", in, existing}, exitUsage, "already exists"},
		{"missing input", []string{"ab", filepath.Join(dir, "nope.png"), out}, exitInput, "nope.png"},
		{"unsupported", []string{"ab", gif, out}, exitInput, "unsupported image format"},
		{"malformed", []string{"ab", broken, out}, exitInput, "png:"},
		{"timeout", []string{"--max-digits", "128", "--timeout", "50ms", strings.Repeat("0", 40), in, out}, exitLimit, "no match within limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, tt.args...)
			if code != tt.code {
				t.Errorf("exit %d, want %d (stderr: %s)", code, tt.code, stderr)
			}
			if !strings.Contains(stderr, tt.msg) {
				t.Errorf("stderr %q does not mention %q", stderr, tt.msg)
			}
			if stdout != "" {
				t.Errorf("unexpected stdout %q", stdout)
			}
			if _, err := os.Stat(out); err == nil {
				t.Error("output written despite error")
			}
		})
	}
	if got, _ := os.ReadFile(existing); string(got) != "keep" {
		t.Error("existing output was overwritten")
	}
}

func TestInterrupted(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.png")
	writeFile(t, in, testimg.PNG(t, 4, 4))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"abcdef", in, filepath.Join(dir, "out.png")}, &stdout, &stderr)
	if code != exitSignaled || !strings.Contains(stderr.String(), "interrupted") {
		t.Errorf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestHelp(t *testing.T) {
	code, _, stderr := runCLI(t, "-h")
	if code != exitOK || !strings.Contains(stderr, "usage: spoof") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}
