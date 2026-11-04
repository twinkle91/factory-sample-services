package cache

import "testing"

func TestKeyUsesPrefix(t *testing.T) {
	v := NewVisits(nil, DefaultPrefix)
	if got, want := v.key("abc123"), "shortlinks:visits:abc123"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
}
