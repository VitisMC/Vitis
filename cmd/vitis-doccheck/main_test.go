package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vitismc/vitis/internal/config"
)

// TestCheck verifies the complete checker against isolated repository fixtures.
func TestCheck(t *testing.T) {
	for _, name := range []string{"valid", "broken link", "missing translation", "missing key", "duplicate key", "stale key", "missing config"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			keys := configKeys(reflect.TypeOf(config.Config{}), "")
			var rows strings.Builder
			for _, key := range keys {
				rows.WriteString("| `" + key + "` | description |\n")
			}
			writeFixture(t, root, "README.md", "[Docs](docs/eng/README.md)\n")
			for _, language := range []string{"eng", "rus"} {
				writeFixture(t, root, "docs/"+language+"/README.md", "[Config](configuration.md)\n")
				writeFixture(t, root, "docs/"+language+"/configuration.md", rows.String())
			}
			switch name {
			case "broken link":
				writeFixture(t, root, "README.md", "[Missing](absent.md)\n")
			case "missing translation":
				writeFixture(t, root, "docs/eng/extra.md", "extra\n")
			case "missing key":
				writeFixture(t, root, "docs/rus/configuration.md", strings.ReplaceAll(rows.String(), "| `"+keys[0]+"` | description |\n", ""))
			case "duplicate key":
				writeFixture(t, root, "docs/eng/configuration.md", rows.String()+"| `"+keys[0]+"` | duplicate |\n")
			case "stale key":
				writeFixture(t, root, "docs/eng/configuration.md", rows.String()+"| `server.removed` | stale |\n")
			case "missing config":
				if err := os.Remove(filepath.Join(root, "docs/rus/configuration.md")); err != nil {
					t.Fatal(err)
				}
			}
			err := check(root)
			if name == "valid" && err != nil {
				t.Fatal(err)
			}
			if name != "valid" && err == nil {
				t.Fatal("accepted invalid documentation")
			}
		})
	}
}

// TestCheckLinks covers relative paths, reference links, URL encoding, and fenced examples.
func TestCheckLinks(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "docs/target file.md", "target\n")
	file := filepath.Join(root, "docs/page.md")
	for _, tc := range []struct {
		name, text string
		valid      bool
	}{
		{"encoded path", "[Target](target%20file.md#section)", true},
		{"angle path", "[Target](<target file.md>)", true},
		{"angle title", "[Target](<target file.md> \"Title\")", true},
		{"title", "[Target](target%20file.md \"Title\")", true},
		{"reference", "[target]: target%20file.md", true},
		{"root relative", "[Target](/docs/target%20file.md)", true},
		{"external", "[Web](https://example.com/path) [Mail](mailto:team@example.com)", true},
		{"fragment", "[Section](#section)", true},
		{"code span", "`[Example](missing.md)` [Real](target%20file.md)", true},
		{"wide code span", "`` `[Example](missing.md)` ``", true},
		{"unmatched code", "` [Missing](missing.md)", false},
		{"fence", "```md\n[Example](missing.md)\n```\n", true},
		{"tilde fence", "~~~~md\n[Example](missing.md)\n~~~\n[Example](missing.md)\n~~~~\n", true},
		{"outside fence", "```md\nexample\n```\n[Missing](missing.md)", false},
		{"broken reference", "[target]: absent.md", false},
		{"image", "![Image](absent.png)", false},
		{"escape", "[Target](../../outside.md)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkLinks(root, file, tc.text)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}

// TestConfigKeys verifies nested YAML tags without depending on a frozen key inventory.
func TestConfigKeys(t *testing.T) {
	type nested struct {
		Limit   int `yaml:"limit,omitempty"`
		Ignored int `yaml:"-"`
	}
	type fixture struct {
		Network nested `yaml:"network"`
		Enabled bool   `yaml:"enabled"`
	}
	got := configKeys(reflect.TypeOf(fixture{}), "")
	if want := []string{"enabled", "network.limit"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys=%v, want=%v", got, want)
	}
	if err := checkConfigRows("```md\n| `enabled` | example |\n```", []string{"enabled"}); err == nil {
		t.Fatal("counted a fenced example as a configuration row")
	}
}

// writeFixture writes a file inside a test-owned repository tree.
func writeFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
