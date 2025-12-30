# Style and conventions

- Language: Go (module `vnt`, go 1.25).
- Formatting: standard `gofmt` conventions (no separate formatter config found).
- General style: idiomatic Go, avoid `interface{}` in favor of `any` where applicable; prefer `[]byte` for map keys/strings internally to reduce allocations (bytes-first APIs).
- File naming: lower_snake or short Go file names, grouped by concern (e.g., `array_ops.go`, `map_ops.go`, `encode.go`).
