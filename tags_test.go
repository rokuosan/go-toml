package toml

import "testing"

func TestParseTag(t *testing.T) {
	tests := []struct {
		tag  string
		name string
		opts tagOptions
	}{
		{tag: "", name: "", opts: ""},
		{tag: "name", name: "name", opts: ""},
		{tag: "name,omitempty", name: "name", opts: "omitempty"},
		{tag: ",omitempty,omitzero", name: "", opts: "omitempty,omitzero"},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			name, opts := parseTag(tt.tag)
			if name != tt.name {
				t.Fatalf("name = %q, want %q", name, tt.name)
			}
			if opts != tt.opts {
				t.Fatalf("opts = %q, want %q", opts, tt.opts)
			}
		})
	}
}

func TestTagOptionsContains(t *testing.T) {
	opts := tagOptions("omitempty,omitzero,string")
	for _, option := range []string{"omitempty", "omitzero", "string"} {
		if !opts.Contains(option) {
			t.Fatalf("expected %q in %q", option, opts)
		}
	}
	for _, option := range []string{"empty", "zero", ""} {
		if opts.Contains(option) {
			t.Fatalf("did not expect %q in %q", option, opts)
		}
	}
}

func TestUnmarshalTagOptionsUseFieldNameWhenTagNameIsEmpty(t *testing.T) {
	var cfg struct {
		Title string `toml:",omitempty"`
	}
	if err := Unmarshal([]byte(`title = "Example"`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Title != "Example" {
		t.Fatalf("Title = %q", cfg.Title)
	}
}

func TestUnmarshalIgnoredFieldTag(t *testing.T) {
	var cfg struct {
		Title string `toml:"-"`
	}
	if err := Unmarshal([]byte(`title = "Example"`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Title != "" {
		t.Fatalf("Title = %q", cfg.Title)
	}
}
