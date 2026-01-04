package jsonschema

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tron "tron"
)

var suiteSkip = []string{
	"ecmascript-regex.json",
	"zeroTerminatedFloats.json",
}

func TestJSONSchemaSuiteDraft2020(t *testing.T) {
	testSuite(t, "./testdata/JSON-Schema-Test-Suite")
}

func testSuite(t *testing.T, suite string) {
	if _, err := os.Stat(suite); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("test suite not found at %s", suite)
		}
		t.Fatal(err)
	}
	testDir(t, suite, "draft2020-12", Draft2020)
}

func testDir(t *testing.T, suite, dpath string, draft *Draft) {
	dir := filepath.Join(suite, "tests", dpath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			testDir(t, suite, filepath.Join(dpath, name), draft)
			continue
		}
		if filepath.Ext(name) != ".json" {
			continue
		}
		if slices.Contains(suiteSkip, name) {
			continue
		}
		testFile(t, suite, filepath.Join(dpath, name), draft)
	}
}

func testFile(t *testing.T, suite, fpath string, draft *Draft) {
	optional := strings.Contains(fpath, string(filepath.Separator)+"optional"+string(filepath.Separator))
	if optional && os.Getenv("TRON_SCHEMA_OPTIONAL") == "" {
		t.Skipf("optional test suite disabled: %s", fpath)
	}
	fullPath := filepath.Join(suite, "tests", fpath)
	t.Logf("file: %s", fullPath)

	file, err := os.Open(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	url := "http://testsuites.com/schema.json"
	var groups []struct {
		Description string `json:"description"`
		Schema      any    `json:"schema"`
		Tests       []struct {
			Description string `json:"description"`
			Data        any    `json:"data"`
			Valid       bool   `json:"valid"`
		} `json:"tests"`
	}
	dec := json.NewDecoder(file)
	dec.UseNumber()
	if err := dec.Decode(&groups); err != nil {
		t.Fatal(err)
	}

	for _, group := range groups {
		t.Log(group.Description)

		c := NewCompiler()
		c.DefaultDraft(draft)
		if optional {
			c.AssertFormat()
			c.AssertContent()
		}
		loader := SchemeURLLoader{
			"file":  FileLoader{},
			"http":  suiteRemotes(suite),
			"https": suiteRemotes(suite),
		}
		c.UseLoader(loader)

		schemaDoc, err := tronDocumentFromAny(group.Schema)
		if err != nil {
			t.Fatalf("schema doc: %v", err)
		}
		if err := c.AddResourceTRON(url, schemaDoc); err != nil {
			t.Fatalf("add resource: %v", err)
		}
		sch, err := c.Compile(url)
		if err != nil {
			t.Fatalf("schema compilation failed: %v", err)
		}
		for _, test := range group.Tests {
			t.Logf("    %s", test.Description)
			dataDoc, err := tronDocumentFromAny(test.Data)
			if err != nil {
				t.Fatalf("data doc: %v", err)
			}
			err = sch.ValidateTRON(dataDoc)
			if got := err == nil; got != test.Valid {
				for _, line := range strings.Split(fmt.Sprintf("%v", err), "\n") {
					t.Logf("        %s", line)
				}
				for _, line := range strings.Split(fmt.Sprintf("%#v", err), "\n") {
					t.Logf("        %s", line)
				}
				if verr, ok := err.(*ValidationError); ok {
					detailed, err := json.MarshalIndent(verr.DetailedOutput(), "", "    ")
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("detailed: %s", string(detailed))
					basic, err := json.MarshalIndent(verr.BasicOutput(), "", "    ")
					if err != nil {
						t.Fatal(err)
					}
					t.Logf("basic: %s", string(basic))
				}
				t.Errorf("        valid: got %v, want %v", got, test.Valid)
				groupSchema, _ := json.Marshal(group.Schema)
				t.Log("schema:", string(groupSchema))
				data, _ := json.Marshal(test.Data)
				t.Log("data:", string(data))
				t.FailNow()
			}
		}
	}
}

type suiteRemotes string

func (rl suiteRemotes) Load(url string) (any, error) {
	if rem, ok := strings.CutPrefix(url, "http://localhost:1234/"); ok {
		return loadRemoteJSON(string(rl), rem)
	}
	if rem, ok := strings.CutPrefix(url, "https://localhost:1234/"); ok {
		return loadRemoteJSON(string(rl), rem)
	}
	return nil, errors.New("no internet")
}

func loadRemoteJSON(root, ref string) (any, error) {
	f, err := os.Open(path.Join(root, "remotes", ref))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return UnmarshalJSON(f)
}

func tronDocumentFromAny(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return tron.FromJSON(raw)
}
