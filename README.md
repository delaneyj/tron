# TRie Object Notation (TRON)

TRie Object Notation (TRON) is a binary format intended to be compatible with JSON primitives while using HAMT (for maps) and vector tries (for arrays) to support fast copy-on-write updates without rewriting the entire document. It targets wire use and embedding as a blob in a JSON column or KV store, not a full database or storage engine.

The name emphasizes the trie-based layout used for maps and arrays.

This repository hosts the evolving spec and a reference implementation in Go.

## Features

- JSON-compatible primitives (null, boolean, number, string, array, object).
- Canonical encoding: same logical value => same bytes.
- Copy-on-write updates with historical roots (append-only trailer).
- Random access without decoding the full document.
- Efficient map/array updates via HAMT + vector trie nodes.
- Stream-friendly: read nodes as needed from a byte slice.
- Go reference implementation includes JSON Patch, JSON Merge Patch, and JMESPath-style queries.

## Status

Work in progress. Expect breaking changes as the spec solidifies.

## Benchmarks

GeoJSON fixture: `implementations/go/tron/testdata/geojson_large.json`. Command: `go test -bench . -benchmem -run=^$ ./...` (run in `implementations/go/tron`) on AMD Ryzen 9 6900HX. Size columns are KB (1024 bytes); zstd uses default settings.

**decode + read**
| Format | ns/op | MB/s | B/op | allocs/op | size (KB) | zstd (KB) |
| --- | --- | --- | --- | --- | --- | --- |
| TRON | 3,093 | 861.41 | 216 | 9 | 2.60 | 0.98 |
| JSON | 65,311 | 33.70 | 11,512 | 345 | 2.15 | 0.48 |
| CBOR | 63,332 | 17.32 | 10,520 | 309 | 1.07 | 0.54 |

**decode + full clone**
| Format | ns/op | MB/s | B/op | allocs/op | size (KB) | zstd (KB) |
| --- | --- | --- | --- | --- | --- | --- |
| TRON | 71,232 | 37.40 | 13,111 | 107 | 2.60 | 0.98 |

**decode + modify + encode**
| Format | ns/op | MB/s | B/op | allocs/op | size (KB) | zstd (KB) |
| --- | --- | --- | --- | --- | --- | --- |
| TRON | 19,697 | 135.25 | 10,562 | 24 | 2.60 | 0.98 |
| JSON | 133,167 | 16.53 | 16,649 | 469 | 2.15 | 0.48 |
| CBOR | 84,967 | 12.91 | 11,719 | 310 | 1.07 | 0.54 |

**encode only**
| Format | ns/op | MB/s | B/op | allocs/op | size (KB) | zstd (KB) |
| --- | --- | --- | --- | --- | --- | --- |
| TRON | 36,976 | 72.05 | 0 | 0 | 2.60 | 0.98 |
| JSON | 48,732 | 45.17 | 5,107 | 124 | 2.15 | 0.48 |
| CBOR | 40,825 | 26.87 | 1,155 | 1 | 1.07 | 0.54 |

**decode + encode**
| Format | ns/op | MB/s | B/op | allocs/op | size (KB) | zstd (KB) |
| --- | --- | --- | --- | --- | --- | --- |
| JSON | 161,909 | 13.59 | 16,653 | 469 | 2.15 | 0.48 |
| CBOR | 110,180 | 9.96 | 11,719 | 310 | 1.07 | 0.54 |

Note: TRON updates are copy-on-write. The modify benchmark only re-encodes nodes along the updated path, while full clone re-encodes the entire tree. Encode-only benchmarks start from a pre-parsed in-memory object (no decode cost). The TRON encode-only benchmark reuses a pooled builder, a dedicated encoder workspace for slice reuse, and appends the trailer in-place to avoid per-iteration buffer allocation/copies.

## Goals

- Represent all JSON primitive types (null, boolean, number, string, array, object).
- Support efficient random access and in-place updates via HAMT and vector trie nodes.
- Be deterministic and canonical for a given logical value (same bytes every time).
- Be streamable for reading without loading everything into memory.
- Keep the format simple enough for multiple implementations.
- Work well as a self-contained blob for transport or database/KV storage.

## Non-goals

- Compression (intended to be paired with streaming compressors like zstd or brotli).
- Schema validation or type enforcement (JSON Schema or other layers can sit on top).
- Arbitrary user-defined types (use MsgPack/CBOR for richer types; TRON stays language/type-system agnostic).
- Acting as a database or primary on-disk storage format.

## Inspiration

TRON was inspired by conversations with the lite3.io author. Those exchanges helped push a rebrand, but there was enough difference and friction in requirements that a distinct format was needed.

## Spec

See `SPEC.md` for the draft binary format and `PRIMER.md` for a HAMT/vector trie overview.

## Reference implementation

The Go implementation lives under `implementations/go/tron` (module `tron`) and tracks the spec as it evolves.

Path queries (JMESPath-style) live under `implementations/go/tron/path`.
JSON Patch (RFC 6902) lives under `implementations/go/tron/patch`.
JSON Merge Patch (RFC 7386) lives under `implementations/go/tron/merge`.

## Contributing

Open an issue or PR with spec suggestions or implementation notes.
