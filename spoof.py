#!/usr/bin/env python3
"""spoof — carry an arbitrary hexstring inside an image, invisibly.

The tool takes an image and a hexstring and writes an adjusted copy of the
image that displays identically in image viewers (README.md:2).  The payload
travels in a PNG ancillary chunk that no viewer renders, so the rendered
pixel data is copied byte-for-byte and visual identity holds by construction
(ADR 0003, N-01).

Python 3, standard library only (ADR 0001).  PNG is the first and currently
the only supported format (ADR 0002); the PNG-specific code below is kept in
one section so a JPEG or GIF adapter can slot in later without touching the
embedder/extractor contracts.
"""

import argparse
import os
import re
import struct
import sys
import tempfile
import zlib

# --- hexstring handling (R-02, R-08) ---------------------------------------

_HEX_DIGITS = re.compile(r"[0-9a-fA-F]+")

# R-10: the payload must fit the carrier.  PNG chunk data is limited to
# 2^31-1 bytes by the 32-bit length field (the high bit must be 0).
MAX_PAYLOAD_BYTES = 2 ** 31 - 1


class SpoofError(Exception):
    """A user-facing failure: bad input, bad image, or payload too large."""


def parse_hexstring(text):
    """Validate a hexstring and return it as bytes (R-08).

    Whitespace is ignored and an optional 0x/0X prefix is accepted; anything
    beyond that — non-hexadecimal characters, an odd number of digits, an
    empty string — raises SpoofError with a message naming the problem.
    """
    digits = "".join(text.split())
    if digits[:2].lower() == "0x":
        digits = digits[2:]
    if not digits:
        raise SpoofError("hexstring is empty")
    if not _HEX_DIGITS.fullmatch(digits):
        raise SpoofError(
            "hexstring contains non-hexadecimal characters "
            "(only 0-9, a-f, A-F are allowed)"
        )
    if len(digits) % 2:
        raise SpoofError(
            "hexstring has an odd length ({}); hex digits must come in pairs"
            .format(len(digits))
        )
    return bytes.fromhex(digits)


# --- PNG format adapter (R-09; ADR 0002) -----------------------------------

PNG_SIGNATURE = b"\x89PNG\r\n\x1a\n"
IHDR = b"IHDR"
IEND = b"IEND"

# The carrier: a private ancillary chunk ("spOf" — lowercase first letter =
# ancillary, lowercase second = private, reserved third bit clear, lowercase
# fourth = safe to copy).  Viewers ignore unknown ancillary chunks, so the
# rendered image is unaffected (ADR 0003).  Its data is the raw payload.
CARRIER_TYPE = b"spOf"

# A PNG chunk length field is an unsigned 32-bit integer with the high bit
# reserved to be 0.
_MAX_CHUNK_LENGTH = 2 ** 31 - 1

# Streaming buffer: the file is copied in blocks, so memory use is constant
# beyond the payload itself (N-04).
_COPY_BUFFER = 1 << 16

SUPPORTED_FORMATS = ("PNG",)


def detect_format(path):
    """Return the container format of *path* ("PNG"), or raise (R-09)."""
    try:
        with open(path, "rb") as f:
            signature = f.read(8)
    except OSError as exc:
        raise SpoofError(
            "cannot read {}: {}".format(path, exc.strerror or exc)
        ) from None
    if signature == PNG_SIGNATURE:
        return "PNG"
    raise SpoofError(
        "{} is not a supported image format (supported: {})"
        .format(path, ", ".join(SUPPORTED_FORMATS))
    )


def _is_valid_chunk_type(chunk_type):
    return len(chunk_type) == 4 and all(
        65 <= byte <= 90 or 97 <= byte <= 122 for byte in chunk_type
    )


def _write_chunk(out, chunk_type, data):
    """Serialize one PNG chunk (length, type, data, CRC) to *out*."""
    out.write(struct.pack(">I", len(data)))
    out.write(chunk_type)
    out.write(data)
    out.write(struct.pack(">I", zlib.crc32(chunk_type + data) & 0xFFFFFFFF))


def _iter_chunks(fin, fout=None, drop=(), collect=()):
    """Stream the chunks of an open PNG file, verifying every CRC (N-06).

    Yields ``(chunk_type, data)`` per chunk, where *data* is the chunk's
    bytes only for types listed in *collect* and ``None`` otherwise.  When
    *fout* is given, every chunk not listed in *drop* is copied to it
    byte-for-byte as it is read, so pixel data passes through untouched.
    Raises SpoofError on a structurally invalid file.  Terminates at IEND
    and rejects trailing bytes after it.
    """
    while True:
        header = fin.read(8)
        if not header:
            raise SpoofError("invalid PNG: file ends without an IEND chunk")
        if len(header) < 8:
            raise SpoofError("invalid PNG: truncated chunk header")
        length, chunk_type = struct.unpack(">I4s", header)
        if length > _MAX_CHUNK_LENGTH:
            raise SpoofError(
                "invalid PNG: chunk {} declares an illegal length {}"
                .format(chunk_type.decode("ascii", "replace"), length)
            )
        if not _is_valid_chunk_type(chunk_type):
            raise SpoofError("invalid PNG: malformed chunk type name")

        write_out = fout is not None and chunk_type not in drop
        keep = chunk_type in collect
        if write_out:
            fout.write(header)
        crc = zlib.crc32(chunk_type)
        collected = bytearray() if keep else None
        remaining = length
        while remaining:
            block = fin.read(min(_COPY_BUFFER, remaining))
            if not block:
                raise SpoofError(
                    "invalid PNG: truncated {} chunk data"
                    .format(chunk_type.decode("ascii", "replace"))
                )
            crc = zlib.crc32(block, crc)
            if write_out:
                fout.write(block)
            if keep:
                collected.extend(block)
            remaining -= len(block)
        stored_crc = fin.read(4)
        if len(stored_crc) < 4:
            raise SpoofError("invalid PNG: truncated chunk CRC")
        if crc != struct.unpack(">I", stored_crc)[0]:
            raise SpoofError(
                "invalid PNG: CRC mismatch in {} chunk"
                .format(chunk_type.decode("ascii", "replace"))
            )
        if write_out:
            fout.write(stored_crc)
        yield chunk_type, (bytes(collected) if keep else None)
        if chunk_type == IEND:
            if fin.read(1):
                raise SpoofError("invalid PNG: trailing data after IEND")
            return


def _require_png(fin, path):
    """Check the 8-byte signature and return it."""
    signature = fin.read(8)
    if signature != PNG_SIGNATURE:
        raise SpoofError("{} is not a PNG file".format(path))
    return signature


def _first_chunk_is_ihdr(chunk_type):
    if chunk_type != IHDR:
        raise SpoofError("invalid PNG: first chunk is not IHDR")


# --- embedder / extractor (R-03, R-05, R-06, R-07) --------------------------

def embed(input_path, payload, output_path):
    """Write *payload* into a copy of the PNG at *input_path*.

    The output is a complete PNG whose pixel data is byte-identical to the
    input's (N-01); the payload rides in a ``spOf`` ancillary chunk placed
    right after IHDR.  The input file is only ever read (R-07).  The output
    is assembled at a temporary sibling path and moved into place only after
    a self-check (seq-embed-hexstring.md steps 10–11), so a failure never
    leaves a corrupt or partial output behind (N-06).
    """
    if len(payload) > MAX_PAYLOAD_BYTES:
        raise SpoofError(
            "payload of {} bytes exceeds the maximum PNG chunk data size "
            "of {} bytes (R-10)".format(len(payload), MAX_PAYLOAD_BYTES)
        )
    if _same_file(input_path, output_path):
        raise SpoofError(
            "output path must differ from the input image (the input is "
            "never modified)"
        )

    directory = os.path.dirname(output_path) or "."
    fd, tmp_path = tempfile.mkstemp(
        dir=directory, prefix=os.path.basename(output_path) + ".", suffix=".tmp"
    )
    os.close(fd)
    try:
        with open(input_path, "rb") as fin, open(tmp_path, "wb") as fout:
            fout.write(_require_png(fin, input_path))
            seen_ihdr = False
            # Any payload already in the input is dropped, so embedding is
            # idempotent: re-embedding replaces, never stacks.
            for chunk_type, _ in _iter_chunks(fin, fout, drop=(CARRIER_TYPE,)):
                if not seen_ihdr:
                    _first_chunk_is_ihdr(chunk_type)
                    seen_ihdr = True
                    _write_chunk(fout, CARRIER_TYPE, payload)
            if not seen_ihdr:
                raise SpoofError("invalid PNG: no IHDR chunk")
        # Self-check (N-07): re-parse the output and confirm the round-trip.
        if extract(tmp_path) != payload:
            raise SpoofError(
                "self-check failed: payload read back from the output does "
                "not match the embedded payload"
            )
        os.replace(tmp_path, output_path)
    finally:
        if os.path.exists(tmp_path):
            try:
                os.remove(tmp_path)
            except OSError:
                pass


def extract(path):
    """Return the payload bytes carried by the PNG at *path* (R-06).

    Raises SpoofError if the file carries no payload or carries more than
    one (an ambiguous, hand-edited file).
    """
    payload = None
    with open(path, "rb") as fin:
        _require_png(fin, path)
        first = True
        for chunk_type, data in _iter_chunks(fin, collect=(CARRIER_TYPE,)):
            if first:
                _first_chunk_is_ihdr(chunk_type)
                first = False
            if chunk_type == CARRIER_TYPE:
                if payload is not None:
                    raise SpoofError(
                        "invalid PNG: more than one {} chunk"
                        .format(CARRIER_TYPE.decode("ascii"))
                    )
                payload = data
    if payload is None:
        raise SpoofError("{} carries no embedded hexstring".format(path))
    return payload


def _same_file(path_a, path_b):
    """True if both paths name the same existing file, or are equal text."""
    if os.path.abspath(path_a) == os.path.abspath(path_b):
        return True
    try:
        return os.path.samefile(path_a, path_b)
    except OSError:
        return False


# --- CLI (the only entry point; c4-container.md) ----------------------------

def default_output_path(input_path):
    """``cat.png`` -> ``cat_spoof.png`` (Q-08 assumption)."""
    stem, extension = os.path.splitext(input_path)
    return "{}_spoof{}".format(stem, extension or ".png")


def build_parser():
    parser = argparse.ArgumentParser(
        prog="spoof",
        description="Carry an arbitrary hexstring inside a PNG image; the "
                    "output file displays identically to the original.",
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    embed_parser = subparsers.add_parser(
        "embed", help="embed a hexstring into an image"
    )
    embed_parser.add_argument("image", help="input image file (PNG)")
    embed_parser.add_argument(
        "hexstring", help="the hexstring to carry, e.g. 0xdeadbeef"
    )
    embed_parser.add_argument(
        "-o", "--output", metavar="PATH",
        help="output path (default: <input>_spoof.png)",
    )
    embed_parser.add_argument(
        "--force", action="store_true",
        help="overwrite the output file if it already exists",
    )

    extract_parser = subparsers.add_parser(
        "extract", help="extract the hexstring carried by an image"
    )
    extract_parser.add_argument("image", help="image file to read (PNG)")
    return parser


def main(argv=None):
    args = build_parser().parse_args(argv)
    try:
        if args.command == "embed":
            payload = parse_hexstring(args.hexstring)
            detect_format(args.image)
            output_path = args.output or default_output_path(args.image)
            if os.path.exists(output_path) and not args.force:
                raise SpoofError(
                    "output file {} already exists; use --force to overwrite"
                    .format(output_path)
                )
            embed(args.image, payload, output_path)
            print(
                "embedded {} bytes ({} hex digits) into {}"
                .format(len(payload), 2 * len(payload), output_path)
            )
        else:  # extract
            detect_format(args.image)
            payload = extract(args.image)
            print(payload.hex())
    except BrokenPipeError:
        # The reader closed the pipe (e.g. `spoof extract x | head`); exit
        # quietly like any other filter command, and keep the interpreter's
        # shutdown flush from raising again.
        devnull = os.open(os.devnull, os.O_WRONLY)
        os.dup2(devnull, sys.stdout.fileno())
        return 1
    except (SpoofError, OSError) as exc:
        print("spoof: error: {}".format(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
