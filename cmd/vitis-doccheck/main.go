// Command vitis-doccheck validates repository documentation without network access.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/vitismc/vitis/internal/config"
)

var (
	inlineLink    = regexp.MustCompile(`\[[^\]\n]*\]\((<[^>\n]+>[^)\n]*|[^)\n]+)\)`)
	referenceLink = regexp.MustCompile(`^\s{0,3}\[[^\]]+\]:\s*(<[^>]+>|\S+)`)
	configRow     = regexp.MustCompile("^\\|\\s*`([^`]+)`\\s*\\|")
)

// main reports all documentation failures and exits unsuccessfully if any exist.
func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	if err := check(*root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("documentation checks passed")
}

// check validates links, translation topology, and configuration table coverage.
func check(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	files, err := markdownFiles(root)
	if err != nil {
		return err
	}
	var failures []error
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		failures = append(failures, checkLinks(root, file, string(data)))
	}
	failures = append(failures, checkTranslations(root))
	keys := configKeys(reflect.TypeOf(config.Config{}), "")
	for _, language := range []string{"eng", "rus"} {
		file := filepath.Join(root, "docs", language, "configuration.md")
		data, err := os.ReadFile(file)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if err := checkConfigRows(string(data), keys); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", file, err))
		}
	}
	return errors.Join(failures...)
}

// markdownFiles collects authored Markdown while excluding downloaded and runtime data.
func markdownFiles(root string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(root, "*.md"))
	if err != nil {
		return nil, err
	}
	for _, directory := range []string{"docs", "internal", "cmd", "pkg", "scripts", "configs", "test"} {
		path := filepath.Join(root, directory)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(path, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			if !entry.IsDir() && filepath.Ext(path) == ".md" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

// proseLines removes fenced blocks so examples do not become documentation links.
func proseLines(document string) []string {
	lines := strings.Split(document, "\n")
	marker := byte(0)
	width := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			count := 0
			for count < len(trimmed) && trimmed[count] == trimmed[0] {
				count++
			}
			if count >= 3 {
				if marker == 0 {
					marker, width = trimmed[0], count
				} else if marker == trimmed[0] && count >= width && strings.TrimSpace(trimmed[count:]) == "" {
					marker = 0
				}
				lines[i] = ""
				continue
			}
		}
		if marker != 0 {
			lines[i] = ""
		}
	}
	return lines
}

// checkLinks checks inline, image, and reference-definition paths, excluding URL fragments.
func checkLinks(root, file, document string) error {
	var failures []error
	for number, line := range proseLines(document) {
		line = withoutCodeSpans(line)
		matches := inlineLink.FindAllStringSubmatch(line, -1)
		if match := referenceLink.FindStringSubmatch(line); match != nil {
			matches = append(matches, match)
		}
		for _, match := range matches {
			destination := strings.TrimSpace(match[1])
			if strings.HasPrefix(destination, "<") {
				destination = destination[1:strings.IndexByte(destination, '>')]
			} else if fields := strings.Fields(destination); len(fields) > 0 {
				destination = fields[0]
			}
			parsed, err := url.Parse(destination)
			if err == nil && (parsed.IsAbs() || parsed.Host != "" || parsed.Path == "") {
				continue
			}
			if err == nil {
				path := filepath.Join(filepath.Dir(file), filepath.FromSlash(parsed.Path))
				if strings.HasPrefix(parsed.Path, "/") {
					path = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(parsed.Path, "/")))
				}
				relative, relativeErr := filepath.Rel(root, path)
				if relativeErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
					err = fmt.Errorf("target escapes repository")
				} else {
					_, err = os.Stat(path)
				}
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("%s:%d: link %q: %w", file, number+1, destination, err))
			}
		}
	}
	return errors.Join(failures...)
}

// withoutCodeSpans masks matched backtick spans without hiding unmatched delimiters.
func withoutCodeSpans(line string) string {
	masked := []byte(line)
	for start := 0; start < len(line); start++ {
		if line[start] != '`' {
			continue
		}
		end := start
		for end < len(line) && line[end] == '`' {
			end++
		}
		width := end - start
		for cursor := end; cursor < len(line); {
			if line[cursor] != '`' {
				cursor++
				continue
			}
			closing := cursor
			for closing < len(line) && line[closing] == '`' {
				closing++
			}
			if closing-cursor == width {
				for i := start; i < closing; i++ {
					masked[i] = ' '
				}
				end = closing
				break
			}
			cursor = closing
		}
		start = end - 1
	}
	return string(masked)
}

// checkTranslations requires identical relative Markdown paths in both language trees.
func checkTranslations(root string) error {
	sets := make([]map[string]bool, 2)
	for i, language := range []string{"eng", "rus"} {
		sets[i] = make(map[string]bool)
		base := filepath.Join(root, "docs", language)
		if err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && filepath.Ext(path) == ".md" {
				relative, err := filepath.Rel(base, path)
				if err != nil {
					return err
				}
				sets[i][relative] = true
			}
			return nil
		}); err != nil {
			return err
		}
	}
	var missing []string
	for i, language := range []string{"rus", "eng"} {
		for page := range sets[i] {
			if !sets[1-i][page] {
				missing = append(missing, language+"/"+page)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("missing translations: %s", strings.Join(missing, ", "))
	}
	return nil
}

// configKeys derives dotted YAML leaf keys from the authoritative configuration type.
func configKeys(configType reflect.Type, prefix string) []string {
	var keys []string
	for i := 0; i < configType.NumField(); i++ {
		field := configType.Field(i)
		tag := strings.Split(field.Tag.Get("yaml"), ",")[0]
		if tag == "" || tag == "-" || !field.IsExported() {
			continue
		}
		key := prefix + tag
		if field.Type.Kind() == reflect.Struct {
			keys = append(keys, configKeys(field.Type, key+".")...)
		} else {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// checkConfigRows requires exactly one table row per YAML key and rejects stale keys.
func checkConfigRows(document string, keys []string) error {
	counts := make(map[string]int)
	for _, line := range proseLines(document) {
		if match := configRow.FindStringSubmatch(line); match != nil {
			counts[match[1]]++
		}
	}
	var problems []string
	for _, key := range keys {
		if counts[key] != 1 {
			problems = append(problems, fmt.Sprintf("%s: expected one row, got %d", key, counts[key]))
		}
		delete(counts, key)
	}
	for key := range counts {
		problems = append(problems, "unknown configuration key: "+key)
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		return fmt.Errorf("configuration coverage: %s", strings.Join(problems, "; "))
	}
	return nil
}
