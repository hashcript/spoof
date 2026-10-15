// Package search brute-forces a nonce whose output digest starts with the
// target prefix. The head of the file is hashed once; every attempt restores
// that SHA-512 midstate and hashes only the nonce and tail (ADR-0003, R-13).
package search

import (
	"context"
	"crypto/sha512"
	"encoding"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/hashcript/spoof/internal/format"
	"github.com/hashcript/spoof/internal/prefix"
)

// ErrStopped is returned when ctx ends (timeout or interrupt) before a match.
var ErrStopped = errors.New("search stopped before a match was found")

// checkEvery is how many attempts a worker makes between cancellation checks.
const checkEvery = 1 << 12

// Result is a winning nonce and the SHA-512 digest of the assembled output.
type Result struct {
	Nonce    uint64
	Digest   [sha512.Size]byte
	Attempts uint64
}

// Search runs workers goroutines (runtime.NumCPU() if workers < 1). Worker k
// tries nonces k, k+N, k+2N, ... The first match wins; with one worker the
// result is the lowest matching nonce, so it is deterministic.
func Search(ctx context.Context, slot format.Slot, p prefix.Prefix, workers int) (Result, error) {
	if workers < 1 {
		workers = runtime.NumCPU()
	}
	h := sha512.New()
	h.Write(slot.Head())
	midstate, err := h.(encoding.BinaryMarshaler).MarshalBinary()
	if err != nil {
		return Result{}, fmt.Errorf("saving hash midstate: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		once     sync.Once
		res      Result
		found    bool
		attempts atomic.Uint64
		wg       sync.WaitGroup
	)
	for k := 0; k < workers; k++ {
		wg.Add(1)
		go func(k uint64) {
			defer wg.Done()
			n, d, ok := work(ctx, slot, p, midstate, k, uint64(workers), &attempts)
			if ok {
				once.Do(func() {
					res, found = Result{Nonce: n, Digest: d}, true
					cancel()
				})
			}
		}(uint64(k))
	}
	wg.Wait()
	res.Attempts = attempts.Load()
	if !found {
		return res, ErrStopped
	}
	return res, nil
}

func work(ctx context.Context, slot format.Slot, p prefix.Prefix, midstate []byte, start, stride uint64, attempts *atomic.Uint64) (uint64, [sha512.Size]byte, bool) {
	h := sha512.New()
	u := h.(encoding.BinaryUnmarshaler)
	var (
		buf    []byte
		digest [sha512.Size]byte
		done   = ctx.Done()
		count  uint64
	)
	defer func() { attempts.Add(count) }()
	for nonce := start; ; nonce += stride {
		if count%checkEvery == 0 {
			select {
			case <-done:
				return 0, digest, false
			default:
			}
		}
		count++
		if err := u.UnmarshalBinary(midstate); err != nil {
			return 0, digest, false
		}
		buf = slot.AppendTail(buf[:0], nonce)
		h.Write(buf)
		h.Sum(digest[:0])
		if p.Matches(digest[:]) {
			return nonce, digest, true
		}
		if nonce > ^uint64(0)-stride {
			return 0, digest, false // nonce space exhausted
		}
	}
}
