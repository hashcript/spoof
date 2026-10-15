package search

import (
	"context"
	"crypto/sha512"
	"errors"
	"testing"
	"time"

	"github.com/hashcript/spoof/internal/format"
	"github.com/hashcript/spoof/internal/prefix"
	"github.com/hashcript/spoof/internal/testimg"
)

func mustPrefix(t *testing.T, s string) prefix.Prefix {
	t.Helper()
	p, err := prefix.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSearchFindsPrefix(t *testing.T) {
	inputs := map[string][]byte{"png": testimg.PNG(t, 16, 16), "jpeg": testimg.JPEG(t, 16, 16)}
	for name, src := range inputs {
		for _, workers := range []int{1, 4} {
			a, err := format.Detect(src)
			if err != nil {
				t.Fatal(err)
			}
			slot, err := a.Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			p := mustPrefix(t, "0x24a")
			res, err := Search(context.Background(), slot, p, workers)
			if err != nil {
				t.Fatalf("%s/%d workers: %v", name, workers, err)
			}
			out := format.Assemble(slot, res.Nonce)
			if d := sha512.Sum512(out); d != res.Digest {
				t.Errorf("%s/%d workers: digest of assembled output %x != reported %x", name, workers, d, res.Digest)
			}
			if !p.Matches(res.Digest[:]) {
				t.Errorf("%s/%d workers: digest %x lacks prefix", name, workers, res.Digest)
			}
			if res.Attempts == 0 {
				t.Errorf("%s/%d workers: unexpected result %+v", name, workers, res)
			}
		}
	}
}

func TestSearchSingleWorkerIsDeterministic(t *testing.T) {
	slot, err := format.PNG{}.Parse(testimg.PNG(t, 8, 8))
	if err != nil {
		t.Fatal(err)
	}
	p := mustPrefix(t, "beef")
	first, err := Search(context.Background(), slot, p, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Search(context.Background(), slot, p, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.Nonce != second.Nonce {
		t.Errorf("nonces differ: %d vs %d", first.Nonce, second.Nonce)
	}
	// One worker tries nonces in order, so no smaller nonce matches.
	if first.Attempts != first.Nonce+1 {
		t.Errorf("attempts %d, nonce %d", first.Attempts, first.Nonce)
	}
}

func TestSearchStopsOnTimeout(t *testing.T) {
	slot, err := format.JPEG{}.Parse(testimg.JPEG(t, 8, 8))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	res, err := Search(ctx, slot, mustPrefix(t, "00000000000000000000000000000000"), 2)
	if !errors.Is(err, ErrStopped) {
		t.Fatalf("err = %v, want ErrStopped", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %s to stop", elapsed)
	}
	if res.Attempts == 0 {
		t.Error("no attempts counted")
	}
}
