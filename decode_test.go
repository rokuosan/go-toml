package toml

import (
	"reflect"
	"testing"
)

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
