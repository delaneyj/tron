# Task completion checklist

- Run `gofmt -w .` if Go files changed.
- Run `go test ./...` for correctness.
- Run `go test -bench . -benchmem -run=^$ ./...` when updating benchmarks (and update `README.md`).
- Update spec/docs (`SPEC.md`, `PRIMER.md`, `README.md`) when behavior changes.
