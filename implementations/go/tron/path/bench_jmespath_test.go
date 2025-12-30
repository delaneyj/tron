package path

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tron "tron"
)

var benchSinkValue tron.Value
var benchSinkExpr *Expr

type benchGroup struct {
	Given any           `json:"given"`
	Cases []benchCaseIn `json:"cases"`
}

type benchCaseIn struct {
	Comment    string `json:"comment"`
	Expression string `json:"expression"`
	Bench      string `json:"bench"`
}

type benchCase struct {
	name       string
	expression string
	benchType  string
	doc        []byte
}

func BenchmarkJMESPath(b *testing.B) {
	cases, err := loadJMESPathBenchmarks()
	if err != nil {
		b.Fatalf("load benchmarks: %v", err)
	}
	for _, bc := range cases {
		bc := bc
		switch bc.benchType {
		case "parse":
			b.Run("Parse/"+bc.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					expr, err := Compile(bc.expression)
					if err != nil {
						b.Fatal(err)
					}
					benchSinkExpr = expr
				}
			})
		case "full":
			b.Run("Full/"+bc.name, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					expr, err := Compile(bc.expression)
					if err != nil {
						b.Fatal(err)
					}
					out, err := expr.Search(bc.doc)
					if err != nil {
						if strings.Contains(err.Error(), "not a TRON-backed value") {
							b.Skip("expression result is not TRON-backed")
						}
						b.Fatal(err)
					}
					benchSinkValue = out
				}
			})
		default:
			b.Run("Unknown/"+bc.name, func(b *testing.B) {
				b.Fatalf("unknown bench type %q", bc.benchType)
			})
		}
	}
}

func loadJMESPathBenchmarks() ([]benchCase, error) {
	path := filepath.Join("testdata", "jmespath", "benchmarks.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var groups []benchGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, err
	}
	out := make([]benchCase, 0, 32)
	for gi, group := range groups {
		var doc []byte
		for _, c := range group.Cases {
			if c.Bench == "full" && doc == nil {
				givenBytes, err := json.Marshal(group.Given)
				if err != nil {
					return nil, err
				}
				doc, err = tron.FromJSON(givenBytes)
				if err != nil {
					return nil, err
				}
			}
			name := benchName(gi, c.Comment, c.Expression)
			out = append(out, benchCase{
				name:       name,
				expression: c.Expression,
				benchType:  c.Bench,
				doc:        doc,
			})
		}
	}
	return out, nil
}

func benchName(group int, comment string, expression string) string {
	base := comment
	if strings.TrimSpace(base) == "" {
		base = expression
	}
	base = strings.TrimSpace(base)
	if base == "" {
		base = fmt.Sprintf("group-%d", group)
	}
	base = strings.ReplaceAll(base, " ", "-")
	base = strings.ReplaceAll(base, "/", "-")
	if len(base) > 64 {
		base = base[:64]
	}
	return fmt.Sprintf("g%d-%s", group, base)
}
