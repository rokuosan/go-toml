package toml

import (
	"reflect"
	"testing"
)

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
