"""Automated tests for spoof — no network, stdlib only (N-07).

Run from the repository root:

    python3 -m unittest discover -s tests
"""

import contextlib
import io
import os
import struct
import sys
import tempfile
import unittest
import zlib

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import spoof  # noqa: E402


def make_chunk(chunk_type, data):
    return (
        struct.pack(">I", len(data))
        + chunk_type
        + data
        + struct.pack(">I", zlib.crc32(chunk_type + data) & 0xFFFFFFFF)
    )


def make_png(width=4, height=4, pixel=(37, 99, 201), extra_chunks=()):
    """Build a minimal, valid 8-bit RGB PNG for testing."""
    ihdr = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)
    row = b"\x00" + bytes(pixel) * width  # filter byte 0 + RGB pixels
    idat = zlib.compress(row * height)
    parts = [spoof.PNG_SIGNATURE, make_chunk(spoof.IHDR, ihdr)]
    parts.extend(make_chunk(ctype, data) for ctype, data in extra_chunks)
    parts.append(make_chunk(b"IDAT", idat))
    parts.append(make_chunk(spoof.IEND, b""))
    return b"".join(parts)


def parse_chunks(data):
    """Split a PNG into [(chunk_type, chunk_data), ...] for assertions."""
    chunks = []
    offset = 8  # past the signature
    while offset < len(data):
        (length,) = struct.unpack(">I", data[offset:offset + 4])
        ctype = data[offset + 4:offset + 8]
        chunks.append((ctype, data[offset + 8:offset + 8 + length]))
        offset += 8 + length + 4
    return chunks


class SpoofTestCase(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.dir = self.tmp.name

    def path(self, name):
        return os.path.join(self.dir, name)

    def write_image(self, name="in.png", **kwargs):
        path = self.path(name)
        with open(path, "wb") as f:
            f.write(make_png(**kwargs))
        return path

    def read_file(self, path):
        with open(path, "rb") as f:
            return f.read()

    def run_cli(self, *argv):
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            status = spoof.main(list(argv))
        return status, out.getvalue(), err.getvalue()


class EmbedExtractTests(SpoofTestCase):
    def test_round_trip(self):
        source = self.write_image()
        output = self.path("out.png")
        payload = bytes.fromhex("00ff10aabbccdeadbeef")
        spoof.embed(source, payload, output)
        self.assertEqual(spoof.extract(output), payload)

    def test_hexstring_round_trip_through_cli(self):
        source = self.write_image()
        status, out, err = self.run_cli(
            "embed", source, "0xDEADBEEF", "-o", self.path("out.png")
        )
        self.assertEqual(status, 0, err)
        status, out, err = self.run_cli("extract", self.path("out.png"))
        self.assertEqual(status, 0, err)
        self.assertEqual(out.strip(), "deadbeef")

    def test_hexstring_whitespace_is_ignored(self):
        payload = spoof.parse_hexstring("de ad\nbe\t ef")
        self.assertEqual(payload, bytes.fromhex("deadbeef"))

    def test_empty_payload_hexstring_rejected(self):
        with self.assertRaises(spoof.SpoofError):
            spoof.parse_hexstring("")

    def test_rendered_data_is_byte_identical(self):
        # N-01: the output must render identically *by construction*, so
        # every chunk except the carrier is byte-identical, in order.
        source = self.write_image(
            extra_chunks=[(b"tEXt", b"Comment\x00hello")]
        )
        output = self.path("out.png")
        spoof.embed(source, bytes.fromhex("1234"), output)

        original = parse_chunks(self.read_file(source))
        adjusted = [
            (ctype, data)
            for ctype, data in parse_chunks(self.read_file(output))
            if ctype != spoof.CARRIER_TYPE
        ]
        self.assertEqual(adjusted, original)

    def test_carrier_sits_after_ihdr_with_valid_crc(self):
        source = self.write_image()
        output = self.path("out.png")
        spoof.embed(source, bytes.fromhex("ab"), output)
        chunks = parse_chunks(self.read_file(output))
        self.assertEqual(chunks[0][0], spoof.IHDR)
        self.assertEqual(chunks[1], (spoof.CARRIER_TYPE, bytes.fromhex("ab")))
        # Every CRC in the output is well-formed: reparsing via the strict
        # iterator (which verifies CRCs) succeeds.
        with open(output, "rb") as f:
            f.read(8)  # past the signature, as internal callers do
            list(spoof._iter_chunks(f, collect=(spoof.CARRIER_TYPE,)))

    def test_input_is_never_modified(self):
        source = self.write_image()
        before = self.read_file(source)
        spoof.embed(source, bytes.fromhex("c0ffee"), self.path("out.png"))
        self.assertEqual(self.read_file(source), before)

    def test_default_output_path(self):
        self.assertEqual(
            spoof.default_output_path("cat.png"), "cat_spoof.png"
        )
        source = self.write_image("cat.png")
        status, out, err = self.run_cli("embed", source, "deadbeef")
        self.assertEqual(status, 0, err)
        self.assertTrue(os.path.exists(self.path("cat_spoof.png")))

    def test_reembedding_replaces_payload(self):
        source = self.write_image()
        first = self.path("first.png")
        second = self.path("second.png")
        spoof.embed(source, bytes.fromhex("aa"), first)
        spoof.embed(first, bytes.fromhex("bbcc"), second)
        self.assertEqual(spoof.extract(second), bytes.fromhex("bbcc"))
        # Exactly one carrier chunk, not two.
        carriers = [
            c for c in parse_chunks(self.read_file(second))
            if c[0] == spoof.CARRIER_TYPE
        ]
        self.assertEqual(len(carriers), 1)

    def test_payload_larger_than_capacity_is_rejected(self):
        # R-10: fail with a clear error rather than truncate.  The limit is
        # 2^31-1 bytes, so the test shrinks it instead of building 2 GiB.
        source = self.write_image()
        original_limit = spoof.MAX_PAYLOAD_BYTES
        spoof.MAX_PAYLOAD_BYTES = 4
        try:
            with self.assertRaises(spoof.SpoofError) as caught:
                spoof.embed(source, bytes.fromhex("0011223344"),
                            self.path("out.png"))
        finally:
            spoof.MAX_PAYLOAD_BYTES = original_limit
        self.assertIn("exceeds", str(caught.exception))
        self.assertFalse(os.path.exists(self.path("out.png")))

    def test_output_equal_to_input_is_refused(self):
        source = self.write_image()
        before = self.read_file(source)
        with self.assertRaises(spoof.SpoofError):
            spoof.embed(source, b"xy", source)
        self.assertEqual(self.read_file(source), before)

    def test_extract_without_payload_fails(self):
        source = self.write_image()
        with self.assertRaises(spoof.SpoofError) as caught:
            spoof.extract(source)
        self.assertIn("no embedded hexstring", str(caught.exception))

    def test_extract_from_multiple_carriers_fails(self):
        # A file with two carrier chunks (the embedder never writes this,
        # but a hand-edited file could) is ambiguous and rejected.
        ihdr = struct.pack(">IIBBBBB", 1, 1, 8, 2, 0, 0, 0)
        image = spoof.PNG_SIGNATURE + b"".join([
            make_chunk(spoof.IHDR, ihdr),
            make_chunk(spoof.CARRIER_TYPE, b"\x01"),
            make_chunk(spoof.CARRIER_TYPE, b"\x02"),
            make_chunk(spoof.IEND, b""),
        ])
        path = self.path("two.png")
        with open(path, "wb") as f:
            f.write(image)
        with self.assertRaises(spoof.SpoofError) as caught:
            spoof.extract(path)
        self.assertIn("more than one", str(caught.exception))


class HexstringValidationTests(SpoofTestCase):
    def test_invalid_hexstrings_are_rejected(self):
        for bad in ("abc",          # odd length
                    "deadbee",      # odd length
                    "deadbeeg",     # non-hex character
                    "0x",           # prefix only, no digits
                    "12 34 zz"):    # non-hex after whitespace removal
            with self.subTest(hexstring=bad):
                with self.assertRaises(spoof.SpoofError):
                    spoof.parse_hexstring(bad)

    def test_valid_hexstrings_are_accepted(self):
        for good, expected in (
            ("deadbeef", b"\xde\xad\xbe\xef"),
            ("DEADBEEF", b"\xde\xad\xbe\xef"),
            ("0xdeadbeef", b"\xde\xad\xbe\xef"),
            ("00", b"\x00"),
            ("0f0f0f0f", b"\x0f\x0f\x0f\x0f"),
        ):
            with self.subTest(hexstring=good):
                self.assertEqual(spoof.parse_hexstring(good), expected)

    def test_invalid_hexstring_leaves_no_output(self):
        source = self.write_image()
        output = self.path("out.png")
        status, out, err = self.run_cli("embed", source, "xyz", "-o", output)
        self.assertEqual(status, 1)
        self.assertIn("error", err)
        self.assertFalse(os.path.exists(output))


class ImageValidationTests(SpoofTestCase):
    def test_unsupported_format_is_rejected(self):
        # R-09: a JPEG signature must be rejected naming supported formats.
        jpeg = self.path("photo.jpg")
        with open(jpeg, "wb") as f:
            f.write(b"\xff\xd8\xff\xe0" + b"\x00" * 64)
        with self.assertRaises(spoof.SpoofError) as caught:
            spoof.detect_format(jpeg)
        self.assertIn("supported", str(caught.exception))
        self.assertIn("PNG", str(caught.exception))

    def test_missing_input_file_is_rejected(self):
        status, out, err = self.run_cli(
            "embed", self.path("nope.png"), "deadbeef"
        )
        self.assertEqual(status, 1)
        self.assertIn("error", err)

    def corrupt_image_test(self, name, corrupter):
        path = self.path(name)
        with open(path, "wb") as f:
            f.write(corrupter(make_png()))
        output = self.path("out.png")
        with self.assertRaises(spoof.SpoofError):
            spoof.embed(path, b"\xaa", output)
        self.assertFalse(os.path.exists(output))

    def test_bad_crc_rejected(self):
        def corrupt(image):
            return image[:-4] + b"\x00\x00\x00\x00"  # IEND CRC
        self.corrupt_image_test("bad_crc.png", corrupt)

    def test_truncated_file_rejected(self):
        self.corrupt_image_test("truncated.png", lambda img: img[:30])

    def test_missing_iend_rejected(self):
        # IEND is the final 12 bytes (8 header + 4 CRC, no data).
        self.corrupt_image_test("no_iend.png", lambda img: img[:-12])

    def test_trailing_data_after_iend_rejected(self):
        self.corrupt_image_test(
            "trailing.png", lambda img: img + b"junkjunk"
        )

    def test_not_a_png_rejected(self):
        random_file = self.path("random.png")
        with open(random_file, "wb") as f:
            f.write(b"definitely not a png")
        with self.assertRaises(spoof.SpoofError):
            spoof.detect_format(random_file)

    def test_first_chunk_not_ihdr_rejected(self):
        ihdr = struct.pack(">IIBBBBB", 1, 1, 8, 2, 0, 0, 0)
        image = spoof.PNG_SIGNATURE + make_chunk(b"tEXt", b"k\x00v") + \
            make_chunk(spoof.IHDR, ihdr) + make_chunk(spoof.IEND, b"")
        path = self.path("weird.png")
        with open(path, "wb") as f:
            f.write(image)
        with self.assertRaises(spoof.SpoofError):
            spoof.extract(path)


class CliTests(SpoofTestCase):
    def test_embed_reports_output_path(self):
        source = self.write_image()
        status, out, err = self.run_cli(
            "embed", source, "deadbeef", "-o", self.path("out.png")
        )
        self.assertEqual(status, 0, err)
        self.assertIn(self.path("out.png"), out)

    def test_refuses_to_overwrite_without_force(self):
        source = self.write_image()
        output = self.path("out.png")
        with open(output, "wb") as f:
            f.write(b"previous contents")
        status, out, err = self.run_cli(
            "embed", source, "deadbeef", "-o", output
        )
        self.assertEqual(status, 1)
        self.assertIn("--force", err)
        self.assertEqual(self.read_file(output), b"previous contents")

    def test_force_overwrites(self):
        source = self.write_image()
        output = self.path("out.png")
        with open(output, "wb") as f:
            f.write(b"previous contents")
        status, out, err = self.run_cli(
            "embed", source, "deadbeef", "-o", output, "--force"
        )
        self.assertEqual(status, 0, err)
        self.assertEqual(spoof.extract(output), bytes.fromhex("deadbeef"))

    def test_extract_failure_exits_nonzero(self):
        source = self.write_image()
        status, out, err = self.run_cli("extract", source)
        self.assertEqual(status, 1)
        self.assertIn("error", err)

    def test_extract_prints_only_the_hexstring(self):
        source = self.write_image()
        out_path = self.path("out.png")
        spoof.embed(source, bytes.fromhex("beef00"), out_path)
        status, out, err = self.run_cli("extract", out_path)
        self.assertEqual(status, 0, err)
        self.assertEqual(out, "beef00\n")


if __name__ == "__main__":
    unittest.main()
