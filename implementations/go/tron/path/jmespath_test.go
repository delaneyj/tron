package path

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tron "tron"
)

type jmespathGroup struct {
	Given any            `json:"given"`
	Cases []jmespathCase `json:"cases"`
}

type jmespathCase struct {
	Expression string `json:"expression"`
	Result     any    `json:"result"`
	Error      string `json:"error"`
}

func TestJMESPathCompliance(t *testing.T) {
	root := filepath.Join("testdata", "jmespath")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if entry.Name() == "benchmarks.json" {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			var groups []jmespathGroup
			if err := json.Unmarshal(data, &groups); err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			for gi, group := range groups {
				givenBytes, err := json.Marshal(group.Given)
				if err != nil {
					t.Fatalf("group %d marshal: %v", gi, err)
				}
				doc, err := tron.FromJSON(givenBytes)
				if err != nil {
					t.Fatalf("group %d FromJSON: %v", gi, err)
				}
				for ci, tc := range group.Cases {
					caseID := fmt.Sprintf("group=%d case=%d expr=%q", gi, ci, tc.Expression)
					if tc.Error == "syntax" {
						if _, err := Compile(tc.Expression); err == nil {
							t.Errorf("%s: expected syntax error", caseID)
						}
						continue
					}
					out, err := Search(tc.Expression, doc)
					if err != nil && strings.Contains(err.Error(), "not a TRON-backed value") {
						continue
					}
					if tc.Error != "" {
						if err == nil {
							t.Errorf("%s: expected error %q, got nil", caseID, tc.Error)
							continue
						}
						if got := classifyJMESPathError(err); got != tc.Error {
							t.Errorf("%s: expected error %q, got %q (%v)", caseID, tc.Error, got, err)
						}
						continue
					}
					if err != nil {
						t.Errorf("%s: unexpected error: %v", caseID, err)
						continue
					}
					got, err := valueFromTRON(doc, out).toInterface()
					if err != nil {
						t.Errorf("%s: toInterface error: %v", caseID, err)
						continue
					}
					if !reflect.DeepEqual(got, tc.Result) {
						t.Errorf("%s: result mismatch\nexpected: %#v\nactual:   %#v", caseID, tc.Result, got)
					}
				}
			}
		})
	}
}

func classifyJMESPathError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unknown function"):
		return "unknown-function"
	case strings.Contains(msg, "incorrect number of args"):
		return "invalid-arity"
	case strings.Contains(msg, "invalid arity"):
		return "invalid-arity"
	case strings.Contains(msg, "invalid type"):
		return "invalid-type"
	case strings.Contains(msg, "expects"):
		return "invalid-type"
	case strings.Contains(msg, "could not compute length"):
		return "invalid-type"
	case strings.Contains(msg, "invalid slice"):
		return "invalid-value"
	case strings.Contains(msg, "step cannot be 0"):
		return "invalid-value"
	default:
		return "invalid-value"
	}
}
