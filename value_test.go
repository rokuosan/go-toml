package toml

import (
	"math"
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
	if got := doc["oct"]; got != int64(0o755) {
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
