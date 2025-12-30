# TRON Path

This package evaluates JMESPath expressions directly against TRON documents. It uses an embedded parser derived from `github.com/jmespath/go-jmespath` and a TRON-aware interpreter, returning `tron.Value` from `Search`.

## Search

Simple lookups:

```go
doc, err := tron.FromJSON([]byte(`{"user":{"name":"Ada","tags":["research","math"]}}`))
if err != nil {
	log.Fatal(err)
}
val, err := path.Search("user.name", doc)
if err != nil {
	log.Fatal(err)
}
if text, ok := val.AsString(); ok {
	fmt.Println(text) // "Ada"
}
```

Filters and projections:

```go
val, err := path.Search("features[?properties.elevation > `1000`][0].properties.name", doc)
if err != nil {
	log.Fatal(err)
}
if text, ok := val.AsString(); ok {
	fmt.Println(text)
}
```

Functions:

```go
val, err := path.Search("length(features)", doc)
if err != nil {
	log.Fatal(err)
}
if val.Type == tron.TypeI64 {
	fmt.Println(val.I64)
}
```

## Compile and reuse

```go
expr, err := path.Compile("features[0].properties.elevation")
if err != nil {
	log.Fatal(err)
}
val, err := expr.Search(doc)
if err != nil {
	log.Fatal(err)
}
if val.Type == tron.TypeI64 {
	fmt.Println(val.I64)
}
```

## Transform

`Transform` applies a function to every matched value and returns a new document.

```go
expr, err := path.Compile("features[0].properties.elevation")
if err != nil {
	log.Fatal(err)
}
updated, err := expr.Transform(doc, func(v tron.Value) (tron.Value, error) {
	if v.Type != tron.TypeI64 {
		return v, nil
	}
	return tron.Value{Type: tron.TypeI64, I64: v.I64 + 10}, nil
})
if err != nil {
	log.Fatal(err)
}
_ = updated
```

## Notes

- `Search` returns TRON-backed values (or computed scalars). Expressions that produce computed arrays or objects return an error.
- `Transform` only traverses values that exist in the TRON document. It supports field/index/slice access, projections, filters, flatten, subexpressions, and pipes. It does not support computed values (functions, literals, multiselects, or comparators).
- Binary values are returned as `tron.TypeBin` with raw bytes.

## Attribution

The embedded lexer/parser in this package (`lexer.go`, `parser.go`, `*_string.go`) is derived from `github.com/jmespath/go-jmespath` (MIT license). See `LICENSE`.

We originally forked `go-jmespath` to add pooling and tighten allocations during parsing. Those changes were small and limited to the lexer/parser, so we inlined the relevant files here to avoid a full fork dependency while keeping the optimizations.
