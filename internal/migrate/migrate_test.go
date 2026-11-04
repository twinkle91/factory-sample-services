package migrate

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadSortsByVersion(t *testing.T) {
	fsys := fstest.MapFS{
		"010_add_index.sql":    {Data: []byte("CREATE INDEX ...")},
		"002_add_column.sql":   {Data: []byte("ALTER TABLE ...")},
		"001_create_links.sql": {Data: []byte("CREATE TABLE ...")},
		"embed.go":             {Data: []byte("package migrations")},
	}
	got, err := Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{1, 2, 10}
	if len(got) != len(want) {
		t.Fatalf("got %d migrations, want %d", len(got), len(want))
	}
	for i, m := range got {
		if m.Version != want[i] {
			t.Errorf("migration %d: version %d, want %d", i, m.Version, want[i])
		}
	}
	if got[0].Name != "create_links" || got[0].SQL != "CREATE TABLE ..." {
		t.Errorf("first migration = %+v", got[0])
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	tests := map[string]struct {
		fsys fstest.MapFS
		want string
	}{
		"no version":        {fstest.MapFS{"create_links.sql": {}}, "name must look like"},
		"zero version":      {fstest.MapFS{"000_init.sql": {}}, "positive number"},
		"duplicate version": {fstest.MapFS{"001_a.sql": {}, "1_b.sql": {}}, "share version 1"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Load(tt.fsys)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}
