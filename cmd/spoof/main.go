// Command spoof writes a copy of a PNG or JPEG image whose SHA-512 digest
// starts with a chosen hexstring and which decodes to identical pixels.
//
//	spoof [flags] <hexstring> <input> <output>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashcript/spoof/internal/format"
	"github.com/hashcript/spoof/internal/output"
	"github.com/hashcript/spoof/internal/prefix"
	"github.com/hashcript/spoof/internal/search"
	"github.com/hashcript/spoof/internal/verify"
)

// Exit codes.
const (
	exitOK       = 0
	exitInput    = 1 // unsupported or malformed input, I/O or verification failure
	exitUsage    = 2
	exitLimit    = 3 // timeout reached
	exitSignaled = 130
)

const (
	maxInputSize     = 256 << 20
	warnDigits       = 10
	defaultMaxDigits = 12
	defaultTimeout   = time.Hour
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("spoof", flag.ContinueOnError)
	fs.SetOutput(stderr)
	timeout := fs.Duration("timeout", defaultTimeout, "give up after this long (0 = no limit)")
	maxDigits := fs.Int("max-digits", defaultMaxDigits, "refuse hexstrings longer than this many digits (at most 128)")
	workers := fs.Int("workers", 0, "number of search goroutines (0 = one per CPU; 1 = deterministic output)")
	force := fs.Bool("force", false, "overwrite the output file if it exists")
	fs.Usage = func() {
		fmt.Fprint(stderr, `usage: spoof [flags] <hexstring> <input> <output>

Writes <output>, a copy of the PNG or JPEG <input> that decodes to identical
pixels but whose SHA-512 digest starts with <hexstring>. Prints the digest.

flags:
`)
		fs.PrintDefaults()
	}

	pos, err := parseInterspersed(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return exitOK
	}
	if err != nil {
		return exitUsage
	}
	if len(pos) != 3 {
		fs.Usage()
		return exitUsage
	}
	hexArg, inPath, outPath := pos[0], pos[1], pos[2]

	p, err := prefix.Parse(hexArg)
	if err != nil {
		return fail(stderr, exitUsage, err)
	}
	if *maxDigits < 1 || *maxDigits > prefix.MaxDigits {
		return fail(stderr, exitUsage, fmt.Errorf("--max-digits must be between 1 and %d", prefix.MaxDigits))
	}
	if p.Len() > *maxDigits {
		return fail(stderr, exitUsage, fmt.Errorf("hexstring has %d digits, more than --max-digits=%d; expected work is 16^%d attempts", p.Len(), *maxDigits, p.Len()))
	}
	if *timeout < 0 || *workers < 0 {
		return fail(stderr, exitUsage, errors.New("--timeout and --workers must not be negative"))
	}
	if err := output.Check(inPath, outPath, *force); err != nil {
		if errors.Is(err, output.ErrSameFile) || errors.Is(err, output.ErrExists) {
			return fail(stderr, exitUsage, err)
		}
		return fail(stderr, exitInput, err)
	}
	if p.Len() > warnDigits {
		fmt.Fprintf(stderr, "spoof: warning: %d digits need about 16^%d attempts; this may take a very long time\n", p.Len(), p.Len())
	}

	src, err := readCapped(inPath)
	if err != nil {
		return fail(stderr, exitInput, err)
	}
	adapter, err := format.Detect(src)
	if err != nil {
		return fail(stderr, exitInput, err)
	}
	slot, err := adapter.Parse(src)
	if err != nil {
		return fail(stderr, exitInput, err)
	}
	if err := verify.CheckDimensions(adapter.Name(), src); err != nil {
		return fail(stderr, exitInput, err)
	}

	sctx := ctx
	if *timeout > 0 {
		var cancel context.CancelFunc
		sctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	res, err := search.Search(sctx, slot, p, *workers)
	if err != nil {
		if ctx.Err() != nil {
			return fail(stderr, exitSignaled, fmt.Errorf("interrupted after %d attempts", res.Attempts))
		}
		return fail(stderr, exitLimit, fmt.Errorf("no match within limit (%d attempts in %s)", res.Attempts, *timeout))
	}

	out := format.Assemble(slot, res.Nonce)
	if err := verify.Output(adapter.Name(), src, out, p, res.Digest); err != nil {
		return fail(stderr, exitInput, fmt.Errorf("verification failed, nothing written: %w", err))
	}
	if err := output.WriteAtomic(outPath, out, *force); err != nil {
		if errors.Is(err, output.ErrExists) {
			return fail(stderr, exitUsage, err)
		}
		return fail(stderr, exitInput, err)
	}
	fmt.Fprintf(stdout, "%x\n", res.Digest)
	return exitOK
}

// parseInterspersed parses flags that may appear before, between or after
// the positional arguments.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func readCapped(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxInputSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxInputSize {
		return nil, fmt.Errorf("%s is larger than the %d MiB limit", path, maxInputSize>>20)
	}
	return data, nil
}

func fail(stderr io.Writer, code int, err error) int {
	fmt.Fprintf(stderr, "spoof: %v\n", err)
	return code
}
