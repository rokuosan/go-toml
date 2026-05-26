package toml

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func TestParseScalars(t *testing.T) {
	doc, err := ParseString(`
title = "TOML Example"
literal = 'C:\Users\nodejs\templates'
unicode = "snowman \u2603"
enabled = true
answer = 42
hex = 0xDE_AD_BE_EF
oct = 0o755
bin = 0b1101
ratio = -1_234.5e+6
infinite = +inf
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := doc["title"]; got != "TOML Example" {
		t.Fatalf("title = %v", got)
	}
	if got := doc["literal"]; got != `C:\Users\nodejs\templates` {
		t.Fatalf("literal = %v", got)
	}
	if got := doc["unicode"]; got != "snowman \u2603" {
		t.Fatalf("unicode = %v", got)
	}
	if got := doc["enabled"]; got != true {
		t.Fatalf("enabled = %v", got)
	}
	if got := doc["answer"]; got != int64(42) {
		t.Fatalf("answer = %v", got)
	}
	if got := doc["hex"]; got != int64(0xDEADBEEF) {
		t.Fatalf("hex = %v", got)
	}
	if got := doc["oct"]; got != int64(0755) {
		t.Fatalf("oct = %v", got)
	}
	if got := doc["bin"]; got != int64(13) {
		t.Fatalf("bin = %v", got)
	}
	if got := doc["ratio"]; got != -1234.5e+6 {
		t.Fatalf("ratio = %v", got)
	}
	if got := doc["infinite"].(float64); !math.IsInf(got, 1) {
		t.Fatalf("infinite = %v", got)
	}
}

func TestParseStrings(t *testing.T) {
	doc, err := ParseString("basic = \"line\\nnext\"\nml = \"\"\"\nfirst\\\n  second\n\"\"\"\nlit = '''one\n''two''\n'''\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := doc["basic"]; got != "line\nnext" {
		t.Fatalf("basic = %q", got)
	}
	if got := doc["ml"]; got != "firstsecond\n" {
		t.Fatalf("ml = %q", got)
	}
	if got := doc["lit"]; got != "one\n''two''\n" {
		t.Fatalf("lit = %q", got)
	}
}

func TestParseTablesArraysAndInlineTables(t *testing.T) {
	doc, err := ParseString(`
[owner]
name = "Tom"

[database.settings]
ports = [
  8000,
  8001, # comment
]
limits = { cpu = 2, memory = "4GiB" }

[[products]]
name = "Hammer"

[[products]]
name = "Nail"
`)
	if err != nil {
		t.Fatal(err)
	}
	owner := doc["owner"].(map[string]any)
	if owner["name"] != "Tom" {
		t.Fatalf("owner.name = %v", owner["name"])
	}
	settings := doc["database"].(map[string]any)["settings"].(map[string]any)
	if !reflect.DeepEqual(settings["ports"], []any{int64(8000), int64(8001)}) {
		t.Fatalf("ports = %#v", settings["ports"])
	}
	if settings["limits"].(map[string]any)["memory"] != "4GiB" {
		t.Fatalf("limits = %#v", settings["limits"])
	}
	products := doc["products"].([]any)
	if products[0].(map[string]any)["name"] != "Hammer" || products[1].(map[string]any)["name"] != "Nail" {
		t.Fatalf("products = %#v", products)
	}
}

func TestParseDateTimes(t *testing.T) {
	doc, err := ParseString(`
odt = 1979-05-27T07:32:00Z
ldt = 1979-05-27 07:32:00.999
ld = 1979-05-27
lt = 07:32:00.123
`)
	if err != nil {
		t.Fatal(err)
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
}

func TestRejectInvalidDocuments(t *testing.T) {
	tests := []string{
		"a = 1\na = 2\n",
		"a = 01\n",
		"a = 1__0\n",
		"a = 1_e2\n",
		"a = \"bad\\q\"\n",
		"a = { b = 1, }\n",
		"a = [1, 2\n",
		"[a]\n[a]\n",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseString(input); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	var cfg struct {
		Title string
		Owner struct {
			Name string `toml:"name"`
		}
		Ports []int `toml:"ports"`
	}
	err := Unmarshal([]byte(`
title = "Example"
ports = [8000, 8001]
[owner]
name = "Tom"
`), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Title != "Example" || cfg.Owner.Name != "Tom" || !reflect.DeepEqual(cfg.Ports, []int{8000, 8001}) {
		t.Fatalf("cfg = %#v", cfg)
	}
}
