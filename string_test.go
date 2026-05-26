package toml

import "testing"

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
