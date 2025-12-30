# TRON JSON Schema (2020-12)

This package ports the core compiler/validator from
`github.com/santhosh-tekuri/jsonschema/v6` (Apache 2.0) and adapts it to
validate TRON documents.

The public API keeps the original package name (`jsonschema`) while living at
`tron/schema`. Use the TRON helpers:

```go
compiler := jsonschema.NewTRONCompiler()
schema, err := compiler.CompileTRON(jsonschema.DefaultTRONSchemaURL, schemaDoc)
if err != nil {
    // handle compile error
}
if err := schema.ValidateTRON(instanceDoc); err != nil {
    // handle validation error
}
```

Current scope:
- Draft 2020-12 keywords are supported via the ported compiler/validator.
- TRON documents only (schemas and instances).
- `$ref` resolution is limited to in-document references and resources added
  with `AddResourceTRON`.
- `$schema` is limited to `https://json-schema.org/draft/2020-12/schema` (and
  the `https://json-schema.org/schema` alias).

TBD:
- External resource loading (`file://`, `http(s)://`) for `$ref`.

For the original documentation and behavior details, see:
https://pkg.go.dev/github.com/santhosh-tekuri/jsonschema/v6
