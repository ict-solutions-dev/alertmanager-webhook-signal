package util

import "testing"

func TestStringDefault(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback string
		want     string
	}{
		{"empty returns fallback", "", "fallback", "fallback"},
		{"non-empty returns value", "value", "fallback", "value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := StringDefault(test.value, test.fallback); got != test.want {
				t.Errorf("StringDefault(%q, %q) = %q, want %q", test.value, test.fallback, got, test.want)
			}
		})
	}
}

func TestTernary(t *testing.T) {
	if got := Ternary(true, "yes", "no"); got != "yes" {
		t.Errorf("Ternary(true) = %q, want %q", got, "yes")
	}
	if got := Ternary(false, "yes", "no"); got != "no" {
		t.Errorf("Ternary(false) = %q, want %q", got, "no")
	}
}

func TestStringInSlice(t *testing.T) {
	list := []string{"alpha", "beta", "gamma"}
	if !StringInSlice("beta", list) {
		t.Error("expected beta to be found in slice")
	}
	if StringInSlice("delta", list) {
		t.Error("did not expect delta to be found in slice")
	}
	if StringInSlice("anything", nil) {
		t.Error("did not expect a match in a nil slice")
	}
}
