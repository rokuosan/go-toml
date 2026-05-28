package toml

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

type textScalar struct {
	value string
}

func (v textScalar) MarshalText() ([]byte, error) {
	return []byte(v.value), nil
}

func TestMarshalScalars(t *testing.T) {
	doc := Document{
		"title":   "TOML Example",
		"enabled": true,
		"answer":  int64(42),
		"ratio":   -1234.5e6,
		"ports":   []int{8000, 8001},
	}
	out, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(out)
	if err != nil {
		t.Fatalf("encoded TOML did not parse: %v\n%s", err, out)
	}
	if parsed["title"] != "TOML Example" || parsed["enabled"] != true || parsed["answer"] != int64(42) {
		t.Fatalf("parsed = %#v", parsed)
	}
	if !reflect.DeepEqual(parsed["ports"], []any{int64(8000), int64(8001)}) {
		t.Fatalf("ports = %#v", parsed["ports"])
	}
}

func TestMarshalEscapesStringsAsTomlBasicStrings(t *testing.T) {
	doc := Document{
		"control":    "nul:\x00 bell:\a vertical:\v del:\x7f",
		"bad\x00key": "value",
		"bad\x7fkey": "value",
	}
	out, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		`control = "nul:\u0000 bell:\u0007 vertical:\u000B del:\u007F"`,
		`"bad\u0000key" = "value"`,
		`"bad\u007Fkey" = "value"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	parsed, err := Parse(out)
	if err != nil {
		t.Fatalf("encoded TOML did not parse: %v\n%s", err, out)
	}
	if parsed["control"] != doc["control"] || parsed["bad\x00key"] != "value" || parsed["bad\x7fkey"] != "value" {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestMarshalFiniteFloatsRemainTomlFloats(t *testing.T) {
	doc := Document{
		"one":  float64(1),
		"zero": float64(0),
		"big":  float64(100),
	}
	out, err := Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		`one = 1.0`,
		`zero = 0.0`,
		`big = 100.0`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	parsed, err := Parse(out)
	if err != nil {
		t.Fatalf("encoded TOML did not parse: %v\n%s", err, out)
	}
	for _, key := range []string{"one", "zero", "big"} {
		if _, ok := parsed[key].(float64); !ok {
			t.Fatalf("%s parsed as %T, want float64", key, parsed[key])
		}
	}
}

func TestMarshalStructTables(t *testing.T) {
	type product struct {
		Name string
		SKU  int64 `toml:"sku"`
	}
	type config struct {
		Title    string
		Created  time.Time
		Database struct {
			Server string
			Ports  []int
		}
		Products []product
	}

	var cfg config
	cfg.Title = "Example"
	cfg.Created = time.Date(2026, 5, 27, 12, 34, 56, 0, time.UTC)
	cfg.Database.Server = "localhost"
	cfg.Database.Ports = []int{8000, 8001}
	cfg.Products = []product{{Name: "Hammer", SKU: 738594937}, {Name: "Nail", SKU: 284758393}}

	out, err := Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		`Title = "Example"`,
		`Created = 2026-05-27T12:34:56Z`,
		`[Database]`,
		`Server = "localhost"`,
		`Ports = [8000, 8001]`,
		`[[Products]]`,
		`sku = 738594937`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}

	parsed, err := Parse(out)
	if err != nil {
		t.Fatalf("encoded TOML did not parse: %v\n%s", err, out)
	}
	if parsed["Title"] != "Example" {
		t.Fatalf("Title = %v", parsed["Title"])
	}
	products := parsed["Products"].([]any)
	if len(products) != 2 {
		t.Fatalf("products length = %d", len(products))
	}
	if got := products[1].(map[string]any)["Name"]; got != "Nail" {
		t.Fatalf("Products[1].Name = %v", got)
	}
}

func TestMarshalArrayTableNestedTablesAsInlineValues(t *testing.T) {
	type dimension struct {
		Length int64
		Width  int64
	}
	type product struct {
		Name       string
		Dimension  dimension
		Variations []dimension
	}
	out, err := Marshal(struct {
		Products []product
	}{
		Products: []product{
			{
				Name:       "Hammer",
				Dimension:  dimension{Length: 10, Width: 4},
				Variations: []dimension{{Length: 8, Width: 3}},
			},
			{
				Name:       "Nail",
				Dimension:  dimension{Length: 2, Width: 1},
				Variations: []dimension{{Length: 3, Width: 1}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		`[[Products]]`,
		`Dimension = { Length = 10, Width = 4 }`,
		`Variations = [{ Length = 8, Width = 3 }]`,
		`Dimension = { Length = 2, Width = 1 }`,
		`Variations = [{ Length = 3, Width = 1 }]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, `[Products.Dimension]`) {
		t.Fatalf("unexpected nested table header in array table:\n%s", text)
	}
	parsed, err := Parse(out)
	if err != nil {
		t.Fatalf("encoded TOML did not parse: %v\n%s", err, out)
	}
	products := parsed["Products"].([]any)
	if len(products) != 2 {
		t.Fatalf("products length = %d", len(products))
	}
	first := products[0].(map[string]any)
	if got := first["Dimension"].(map[string]any)["Length"]; got != int64(10) {
		t.Fatalf("Products[0].Dimension.Length = %v", got)
	}
}

func TestMarshalQuotedKeys(t *testing.T) {
	out, err := Marshal(Document{
		"quoted key": map[string]any{
			"with spaces": "value",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); !strings.Contains(got, `["quoted key"]`) || !strings.Contains(got, `"with spaces" = "value"`) {
		t.Fatalf("unexpected output:\n%s", got)
	}
	if _, err := Parse(out); err != nil {
		t.Fatalf("encoded TOML did not parse: %v\n%s", err, out)
	}
}

func TestMarshalRejectsUnsupportedTopLevel(t *testing.T) {
	if _, err := Marshal("value"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMarshalRejectsDuplicateStructKeys(t *testing.T) {
	_, err := Marshal(struct {
		First  string `toml:"value"`
		Second string `toml:"value"`
	}{
		First:  "one",
		Second: "two",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `duplicate struct field key "value"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestMarshalTextMarshalerStructAsScalar(t *testing.T) {
	out, err := Marshal(struct {
		Value textScalar
		List  []textScalar
	}{
		Value: textScalar{value: "scalar"},
		List:  []textScalar{{value: "one"}, {value: "two"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		`Value = "scalar"`,
		`List = ["one", "two"]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if _, err := Parse(out); err != nil {
		t.Fatalf("encoded TOML did not parse: %v\n%s", err, out)
	}
}

func TestMarshalRejectsUintOutsideTomlRange(t *testing.T) {
	_, err := Marshal(Document{"value": uint64(math.MaxInt64) + 1})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMarshalRejectsInvalidLocalTime(t *testing.T) {
	for _, value := range []LocalTime{
		{Duration: -time.Nanosecond},
		{Duration: 24 * time.Hour},
	} {
		_, err := Marshal(Document{"value": value})
		if err == nil {
			t.Fatalf("expected error for %v", value.Duration)
		}
		if !strings.Contains(err.Error(), "outside 00:00:00") {
			t.Fatalf("error = %v", err)
		}
	}
}

func TestMarshalMixedArrayWithInlineTable(t *testing.T) {
	out, err := Marshal(Document{
		"items": []any{
			map[string]any{"name": "Hammer"},
			"loose",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(out); !strings.Contains(got, `items = [{ name = "Hammer" }, "loose"]`) {
		t.Fatalf("unexpected output:\n%s", got)
	}
	if _, err := Parse(out); err != nil {
		t.Fatalf("encoded TOML did not parse: %v\n%s", err, out)
	}
}
