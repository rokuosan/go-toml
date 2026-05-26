package benchmarks

import (
	"testing"

	burntsushi "github.com/BurntSushi/toml"
	pelletier "github.com/pelletier/go-toml/v2"
	gotoml "github.com/rokuosan/go-toml"
)

var benchmarkDocument = []byte(`
title = "TOML Benchmark"
enabled = true
answer = 42
ratio = -1_234.5e+6
created = 2026-05-27T12:34:56Z

[owner]
name = "Roku"
organization = "Example"
bio = """
TOML benchmark fixture.
This keeps strings, arrays, inline tables, and array tables in one document.
"""

[database]
server = "192.168.1.1"
ports = [8000, 8001, 8002]
connection_max = 5000
enabled = true
settings = { cpu = 2, memory = "4GiB", disk = "40GiB" }

[clients]
data = [["gamma", "delta"], [1, 2]]
hosts = [
  "alpha",
  "omega",
]

[[products]]
name = "Hammer"
sku = 738594937
color = "gray"

[[products]]
name = "Nail"
sku = 284758393
color = "silver"

[[products]]
name = "Screwdriver"
sku = 284758394
color = "yellow"
`)

type benchmarkConfig struct {
	Title   string
	Enabled bool
	Answer  int64
	Ratio   float64
	Owner   struct {
		Name         string
		Organization string
		Bio          string
	}
	Database struct {
		Server        string
		Ports         []int
		ConnectionMax int `toml:"connection_max"`
		Enabled       bool
		Settings      struct {
			CPU    int `toml:"cpu"`
			Memory string
			Disk   string
		}
	}
	Clients struct {
		Data  [][]any
		Hosts []string
	}
	Products []struct {
		Name  string
		SKU   int64 `toml:"sku"`
		Color string
	}
}

func BenchmarkParse(b *testing.B) {
	for b.Loop() {
		if _, err := gotoml.Parse(benchmarkDocument); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBurntSushiDecodeMap(b *testing.B) {
	for b.Loop() {
		var out map[string]any
		if err := burntsushi.Unmarshal(benchmarkDocument, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPelletierDecodeMap(b *testing.B) {
	for b.Loop() {
		var out map[string]any
		if err := pelletier.Unmarshal(benchmarkDocument, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	for b.Loop() {
		var out benchmarkConfig
		if err := gotoml.Unmarshal(benchmarkDocument, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBurntSushiUnmarshal(b *testing.B) {
	for b.Loop() {
		var out benchmarkConfig
		if err := burntsushi.Unmarshal(benchmarkDocument, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPelletierUnmarshal(b *testing.B) {
	for b.Loop() {
		var out benchmarkConfig
		if err := pelletier.Unmarshal(benchmarkDocument, &out); err != nil {
			b.Fatal(err)
		}
	}
}
