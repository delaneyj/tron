package patch

import "fmt"

// ApplyPatch applies RFC 6902 JSON Patch semantics to a target document.
func ApplyPatch(target, patch []byte) ([]byte, error) {
	return nil, fmt.Errorf("patch: not implemented")
}
