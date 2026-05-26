package toml

import "testing"

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
