package toml

import (
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseDocumentRoundTripFixtures(t *testing.T) {
	files, err := fs.Glob(parserFixtures, "testdata/document/*.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no document fixtures found")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := parserFixtures.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := ParseDocument(data)
			if err != nil {
				t.Fatal(err)
			}
			if got := doc.String(); got != string(data) {
				t.Fatalf("String() changed source:\n%s", got)
			}
		})
	}
}

func TestParseDocumentRoundTripCRLF(t *testing.T) {
	input := "# comment\r\n\r\n[server]\r\nport   = 8000 # keep\r\n"
	doc, err := ParseDocumentString(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.String(); got != input {
		t.Fatalf("String() changed CRLF source:\n%q", got)
	}
	nodes := doc.Nodes()
	if len(nodes) != 4 {
		t.Fatalf("node count = %d, want 4: %#v", len(nodes), nodes)
	}
	for i, node := range nodes {
		if node.Raw != input[node.Start:node.End] {
			t.Fatalf("nodes[%d].Raw does not match span", i)
		}
	}
}

func TestParseDocumentRoundTripTrailingWhitespace(t *testing.T) {
	input := "title = \"Example\"  \n\t# comment with leading tab  \n"
	doc, err := ParseDocumentString(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.String(); got != input {
		t.Fatalf("String() changed trailing whitespace:\n%q", got)
	}
}

func TestParseDocumentAcceptsValidFixtures(t *testing.T) {
	files, err := fs.Glob(parserFixtures, "testdata/valid/*.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := parserFixtures.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := ParseDocument(data)
			if err != nil {
				t.Fatal(err)
			}
			if got := doc.String(); got != string(data) {
				t.Fatalf("String() changed source:\n%s", got)
			}
		})
	}
}

func TestParseDocumentRejectsInvalidFixtures(t *testing.T) {
	files, err := fs.Glob(parserFixtures, "testdata/invalid/*.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := parserFixtures.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseDocument(data); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseDocumentNodes(t *testing.T) {
	input := "# top\n\n[server]\nport   = 8000 # keep\n\n[[products]]\nname = \"Hammer\"\n"
	doc, err := ParseDocumentString(input)
	if err != nil {
		t.Fatal(err)
	}
	nodes := doc.Nodes()
	wantKinds := []SyntaxKind{
		SyntaxComment,
		SyntaxBlankLine,
		SyntaxTable,
		SyntaxKeyValue,
		SyntaxBlankLine,
		SyntaxArrayTable,
		SyntaxKeyValue,
	}
	if len(nodes) != len(wantKinds) {
		t.Fatalf("node count = %d, want %d: %#v", len(nodes), len(wantKinds), nodes)
	}
	for i, want := range wantKinds {
		if nodes[i].Kind != want {
			t.Fatalf("nodes[%d].Kind = %v, want %v", i, nodes[i].Kind, want)
		}
		if nodes[i].Raw != input[nodes[i].Start:nodes[i].End] {
			t.Fatalf("nodes[%d].Raw does not match span", i)
		}
	}
	if !reflect.DeepEqual(nodes[2].Key, []string{"server"}) {
		t.Fatalf("table key = %#v", nodes[2].Key)
	}
	if nodes[2].RawKey != "server" {
		t.Fatalf("table raw key = %q", nodes[2].RawKey)
	}
	if !reflect.DeepEqual(nodes[3].Key, []string{"port"}) {
		t.Fatalf("key/value key = %#v", nodes[3].Key)
	}
	if nodes[3].RawKey != "port" {
		t.Fatalf("key/value raw key = %q", nodes[3].RawKey)
	}
	if nodes[3].RawValue != "8000" {
		t.Fatalf("key/value raw value = %q", nodes[3].RawValue)
	}
	if nodes[3].RawValue != input[nodes[3].ValueStart:nodes[3].ValueEnd] {
		t.Fatal("key/value raw value does not match value span")
	}
	if !reflect.DeepEqual(nodes[5].Key, []string{"products"}) {
		t.Fatalf("array table key = %#v", nodes[5].Key)
	}

	nodes[2].Key[0] = "mutated"
	if got := doc.Nodes()[2].Key[0]; got != "server" {
		t.Fatalf("Nodes returned mutable key backing array: %q", got)
	}
}

func TestParseDocumentDecodedDocument(t *testing.T) {
	doc, err := ParseDocumentString("title = \"Example\"\n[server]\nport = 8000\n")
	if err != nil {
		t.Fatal(err)
	}
	decoded := doc.Document()
	if decoded["title"] != "Example" {
		t.Fatalf("title = %v", decoded["title"])
	}
	server := decoded["server"].(map[string]any)
	if server["port"] != int64(8000) {
		t.Fatalf("server.port = %v", server["port"])
	}
	server["port"] = int64(9000)
	if got := doc.Document()["server"].(map[string]any)["port"]; got != int64(8000) {
		t.Fatalf("Document returned mutable map: %v", got)
	}
}

func TestParseDocumentValueSpans(t *testing.T) {
	input := "\"server.port\" = 8000\nports = [\n  8000,\n  8001, # array comment\n]\nbio = { age = 42, active = true }\n"
	doc, err := ParseDocumentString(input)
	if err != nil {
		t.Fatal(err)
	}
	nodes := doc.Nodes()
	if len(nodes) != 3 {
		t.Fatalf("node count = %d, want 3: %#v", len(nodes), nodes)
	}
	if got := nodes[0].RawKey; got != `"server.port"` {
		t.Fatalf("quoted raw key = %q", got)
	}
	if !reflect.DeepEqual(nodes[0].Key, []string{"server.port"}) {
		t.Fatalf("quoted key = %#v", nodes[0].Key)
	}
	if got := nodes[1].RawValue; got != "[\n  8000,\n  8001, # array comment\n]" {
		t.Fatalf("array raw value = %q", got)
	}
	if got := nodes[2].RawValue; got != "{ age = 42, active = true }" {
		t.Fatalf("inline table raw value = %q", got)
	}
}

func TestParseDocumentDistinguishesQuotedDottedTableKeys(t *testing.T) {
	input := "[\"a.b\"]\nx = 1\n\n[a.b]\ny = 2\n"
	doc, err := ParseDocumentString(input)
	if err != nil {
		t.Fatal(err)
	}
	nodes := doc.Nodes()
	if !reflect.DeepEqual(nodes[0].Key, []string{"a.b"}) {
		t.Fatalf("quoted table key = %#v", nodes[0].Key)
	}
	if !reflect.DeepEqual(nodes[3].Key, []string{"a", "b"}) {
		t.Fatalf("dotted table key = %#v", nodes[3].Key)
	}
}

func TestParseDocumentRejectsInvalidToml(t *testing.T) {
	if _, err := ParseDocumentString("a = 1\na = 2\n"); err == nil {
		t.Fatal("expected duplicate key error")
	}
	if _, err := ParseDocument([]byte{0xff}); err == nil {
		t.Fatal("expected invalid UTF-8 error")
	}
}
