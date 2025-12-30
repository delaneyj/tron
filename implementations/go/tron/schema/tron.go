// TRON adapters for compiling and validating schema documents.
package jsonschema

import (
	"encoding/base64"
	"fmt"

	tron "tron"
)

const DefaultTRONSchemaURL = "mem://schema"

// NewTRONCompiler returns a compiler configured to reject external references.
func NewTRONCompiler() *Compiler {
	c := NewCompiler()
	c.UseLoader(tronLoader{})
	return c
}

// CompileTRON compiles a TRON schema using DefaultTRONSchemaURL.
func CompileTRON(doc []byte) (*Schema, error) {
	return NewTRONCompiler().CompileTRON(DefaultTRONSchemaURL, doc)
}

// CompileTRON compiles a TRON schema at the given URL.
func (c *Compiler) CompileTRON(url string, doc []byte) (*Schema, error) {
	if err := c.AddResourceTRON(url, doc); err != nil {
		return nil, err
	}
	c.UseLoader(tronLoader{})
	return c.Compile(url)
}

// AddResourceTRON registers a TRON schema for in-document reference resolution.
func (c *Compiler) AddResourceTRON(url string, doc []byte) error {
	value, err := tronDocumentToAny(doc)
	if err != nil {
		return err
	}
	return c.AddResource(url, value)
}

// ValidateTRON validates a TRON document against the compiled schema.
func (sch *Schema) ValidateTRON(doc []byte) error {
	value, err := tronDocumentToAny(doc)
	if err != nil {
		return err
	}
	return sch.Validate(value)
}

type tronLoader struct{}

func (tronLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("external schema loading not supported: %s", url)
}

func tronDocumentToAny(doc []byte) (any, error) {
	docType, err := tron.DetectDocType(doc)
	if err != nil {
		return nil, err
	}
	switch docType {
	case tron.DocScalar:
		val, err := tron.DecodeScalarDocument(doc)
		if err != nil {
			return nil, err
		}
		return tronValueToAny(doc, val)
	case tron.DocTree:
		tr, err := tron.ParseTrailer(doc)
		if err != nil {
			return nil, err
		}
		header, _, err := tron.NodeSliceAt(doc, tr.RootOffset)
		if err != nil {
			return nil, err
		}
		switch header.KeyType {
		case tron.KeyMap:
			return tronMapToAny(doc, tr.RootOffset)
		case tron.KeyArr:
			return tronArrayToAny(doc, tr.RootOffset)
		default:
			return nil, fmt.Errorf("unsupported root node type: %d", header.KeyType)
		}
	default:
		return nil, fmt.Errorf("unsupported document type")
	}
}

func tronValueToAny(doc []byte, v tron.Value) (any, error) {
	switch v.Type {
	case tron.TypeNil:
		return nil, nil
	case tron.TypeBit:
		return v.Bool, nil
	case tron.TypeI64:
		return v.I64, nil
	case tron.TypeF64:
		return v.F64, nil
	case tron.TypeTxt:
		return string(v.Bytes), nil
	case tron.TypeBin:
		return "b64:" + base64.StdEncoding.EncodeToString(v.Bytes), nil
	case tron.TypeArr:
		return tronArrayToAny(doc, v.Offset)
	case tron.TypeMap:
		return tronMapToAny(doc, v.Offset)
	default:
		return nil, fmt.Errorf("unknown TRON value type: %d", v.Type)
	}
}

func tronMapToAny(doc []byte, off uint32) (map[string]any, error) {
	out := make(map[string]any)
	if err := tronMapFill(doc, off, out); err != nil {
		return nil, err
	}
	return out, nil
}

func tronMapFill(doc []byte, off uint32, out map[string]any) error {
	h, node, err := tron.NodeSliceAt(doc, off)
	if err != nil {
		return err
	}
	if h.KeyType != tron.KeyMap {
		return fmt.Errorf("node is not a map")
	}
	if h.Kind == tron.NodeLeaf {
		leaf, err := tron.ParseMapLeafNode(node)
		if err != nil {
			return err
		}
		defer tron.ReleaseMapLeafNode(&leaf)
		for _, entry := range leaf.Entries {
			val, err := tronValueToAny(doc, entry.Value)
			if err != nil {
				return err
			}
			out[string(entry.Key)] = val
		}
		return nil
	}
	branch, err := tron.ParseMapBranchNode(node)
	if err != nil {
		return err
	}
	defer tron.ReleaseMapBranchNode(&branch)
	for _, child := range branch.Children {
		if err := tronMapFill(doc, child, out); err != nil {
			return err
		}
	}
	return nil
}

func tronArrayToAny(doc []byte, off uint32) ([]any, error) {
	length, err := tron.ArrayRootLength(doc, off)
	if err != nil {
		return nil, err
	}
	out := make([]any, length)
	for i := uint32(0); i < length; i++ {
		val, ok, err := tron.ArrGet(doc, off, i)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("array index missing: %d", i)
		}
		conv, err := tronValueToAny(doc, val)
		if err != nil {
			return nil, err
		}
		out[i] = conv
	}
	return out, nil
}
