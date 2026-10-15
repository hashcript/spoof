package output

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.png")
	if err := os.WriteFile(in, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dir, "existing.png")
	if err := os.WriteFile(existing, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.png")
	if err := os.Symlink(in, link); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	hard := filepath.Join(dir, "hard.png")
	if err := os.Link(in, hard); err != nil {
		t.Skip("hard links unsupported:", err)
	}

	tests := []struct {
		out   string
		force bool
		want  error
	}{
		{filepath.Join(dir, "out.png"), false, nil},
		{in, true, ErrSameFile},
		{filepath.Join(dir, ".", "in.png"), true, ErrSameFile},
		{link, true, ErrSameFile},
		{hard, true, ErrSameFile},
		{existing, false, ErrExists},
		{existing, true, nil},
	}
	for _, tt := range tests {
		if err := Check(in, tt.out, tt.force); !errors.Is(err, tt.want) {
			t.Errorf("Check(%q, force=%v) = %v, want %v", tt.out, tt.force, err, tt.want)
		}
	}
	if err := Check(filepath.Join(dir, "missing.png"), filepath.Join(dir, "o.png"), false); err == nil {
		t.Error("missing input: want error")
	}
	if err := Check(in, filepath.Join(dir, "nodir", "o.png"), false); err == nil {
		t.Error("missing output directory: want error")
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.png")
	if err := WriteAtomic(out, []byte("first"), false); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(out, []byte("second"), false); !errors.Is(err, ErrExists) {
		t.Errorf("err = %v, want ErrExists", err)
	}
	if err := WriteAtomic(out, []byte("third"), true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "third" {
		t.Errorf("content = %q, %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestWriteAtomicReplacesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.png")
	if err := os.Symlink(target, out); err != nil {
		t.Skip("symlinks unsupported:", err)
	}
	if err := WriteAtomic(out, []byte("new"), true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "keep" {
		t.Errorf("symlink target was overwritten: %q", got)
	}
	if fi, _ := os.Lstat(out); fi.Mode()&os.ModeSymlink != 0 {
		t.Error("output is still a symlink")
	}
}
