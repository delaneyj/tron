# VNT project overview

- Purpose: VonNeumannTries (VNT) is a binary format compatible with JSON primitives, using HAMT for maps and vector tries for arrays to enable fast copy-on-write updates without rewriting whole documents. It targets wire use and embedding as a blob in a JSON column or KV store, not a database engine.
- Docs: `README.md` (summary + benchmarks), `SPEC.md` (binary format draft), `PRIMER.md` (HAMT/vector trie overview).
- Reference implementation: Go, located at `implementations/go/vnt`.
- High-level structure: core encoding/decoding and node types in `implementations/go/vnt/*.go`; path/JMESPath querying in `implementations/go/vnt/path`; test fixtures under `implementations/go/vnt/testdata`.
