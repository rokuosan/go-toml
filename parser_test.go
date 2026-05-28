package toml

import (
	"embed"
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

//go:embed testdata/valid/*.toml testdata/invalid/*.toml testdata/document/*.toml
var parserFixtures embed.FS

func TestParseValidFixtures(t *testing.T) {
	files, err := fs.Glob(parserFixtures, "testdata/valid/*.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no valid fixtures found")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := parserFixtures.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Parse(data); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestParseCompleteFixture(t *testing.T) {
	data, err := parserFixtures.ReadFile("testdata/valid/complete.toml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	if got := doc["title"]; got != "TOML Example" {
		t.Fatalf("title = %v", got)
	}
	if got := doc["answer"]; got != int64(42) {
		t.Fatalf("answer = %v", got)
	}
	if got := doc["ratio"]; got != -1234.5e+6 {
		t.Fatalf("ratio = %v", got)
	}
	if _, ok := doc["odt"].(time.Time); !ok {
		t.Fatalf("odt type = %T", doc["odt"])
	}
	if _, ok := doc["ldt"].(LocalDateTime); !ok {
		t.Fatalf("ldt type = %T", doc["ldt"])
	}
	if _, ok := doc["ld"].(LocalDate); !ok {
		t.Fatalf("ld type = %T", doc["ld"])
	}
	if got := doc["lt"].(LocalTime).Duration; got != 7*time.Hour+32*time.Minute+123*time.Millisecond {
		t.Fatalf("lt = %v", got)
	}
	if got := doc["multiline"]; got != "firstsecond\n" {
		t.Fatalf("multiline = %q", got)
	}
	if got := doc["literal_multiline"]; got != "one\n''two''\n" {
		t.Fatalf("literal_multiline = %q", got)
	}

	dotted := doc["dotted"].(map[string]any)
	if got := dotted["key"].(map[string]any)["value"]; got != "created through a dotted key" {
		t.Fatalf("dotted.key.value = %v", got)
	}
	quoted := doc["quoted key"].(map[string]any)
	if got := quoted["with spaces"]; got != "quoted" {
		t.Fatalf(`"quoted key"."with spaces" = %v`, got)
	}

	owner := doc["owner"].(map[string]any)
	if got := owner["bio"].(map[string]any)["age"]; got != int64(42) {
		t.Fatalf("owner.bio.age = %v", got)
	}
	settings := doc["database"].(map[string]any)["settings"].(map[string]any)
	if !reflect.DeepEqual(settings["ports"], []any{int64(8000), int64(8001), int64(8002)}) {
		t.Fatalf("ports = %#v", settings["ports"])
	}
	products := doc["products"].([]any)
	if len(products) != 2 {
		t.Fatalf("products length = %d", len(products))
	}
	if got := products[1].(map[string]any)["name"]; got != "Nail" {
		t.Fatalf("products[1].name = %v", got)
	}
}

func TestParseInvalidFixtures(t *testing.T) {
	files, err := fs.Glob(parserFixtures, "testdata/invalid/*.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no invalid fixtures found")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := parserFixtures.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Parse(data); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseNewlineHandling(t *testing.T) {
	if _, err := ParseString("a = 1\r\nb = 2\r\n"); err != nil {
		t.Fatalf("CRLF document returned error: %v", err)
	}
	if _, err := ParseString("a = 1\rb = 2"); err == nil {
		t.Fatal("expected bare carriage return to fail")
	}
}

func TestParseDistinguishesQuotedDottedTableKeys(t *testing.T) {
	doc, err := ParseString("[\"a.b\"]\nx = 1\n\n[a.b]\ny = 2\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := doc["a.b"].(map[string]any)["x"]; got != int64(1) {
		t.Fatalf(`"a.b".x = %v`, got)
	}
	if got := doc["a"].(map[string]any)["b"].(map[string]any)["y"]; got != int64(2) {
		t.Fatalf("a.b.y = %v", got)
	}
}
