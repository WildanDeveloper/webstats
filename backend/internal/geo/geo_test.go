package geo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFailures(t *testing.T) {
	for _, name := range []string{"country", "asn"} {
		t.Run(name, func(t *testing.T) {
			load := func(path string) (bool, error) {
				if name == "country" {
					r, err := Load(path)
					return r != nil && r.Loaded(), err
				}
				r, err := LoadASN(path)
				return r != nil && r.Loaded(), err
			}
			if loaded, err := load(""); loaded || err != nil {
				t.Fatal("disabled dataset must remain unloaded without error")
			}
			dir := t.TempDir()
			if loaded, err := load(filepath.Join(dir, "missing.csv")); loaded || err == nil {
				t.Error("missing dataset must return an error")
			}
			for _, content := range []string{"", "invalid", "1.1.1.0/24,AU\n" + strings.Repeat("x", 70000)} {
				path := filepath.Join(dir, "dataset.csv")
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				if loaded, err := load(path); loaded || err == nil {
					t.Error("empty, invalid or truncated dataset must return an error and remain unloaded")
				}
			}
			path := filepath.Join(dir, "valid.csv")
			if err := os.WriteFile(path, []byte("1.1.1.0/24,AU\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if loaded, err := load(path); !loaded || err != nil {
				t.Fatalf("valid dataset: loaded=%v err=%v", loaded, err)
			}
		})
	}
}
