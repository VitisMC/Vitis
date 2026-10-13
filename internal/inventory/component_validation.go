package inventory

import (
	"fmt"
	"strings"
	"unicode/utf16"

	dc "github.com/vitismc/vitis/internal/data/generated/data_component"
)

// validateStructuredFields enforces codec constraints after bounded wire decoding.
func validateStructuredFields(id int32, value any, path string) error {
	switch v := value.(type) {
	case RegistryHolder:
		if v.ID < 0 {
			return validateStructuredFields(id, v.Data, path)
		}
	case map[string]any:
		for name, field := range v {
			if err := validateStructuredFields(id, field, path+"/"+name); err != nil {
				return err
			}
		}
	case []any:
		if (id == dc.ComponentWrittenBookContent || id == dc.ComponentWritableBookContent) && path == "/pages" && len(v) > 100 {
			return fmt.Errorf("book has too many pages")
		}
		for _, field := range v {
			if err := validateStructuredFields(id, field, path+"/entry"); err != nil {
				return err
			}
		}
	case int32:
		if strings.HasSuffix(path, "/comparatorOutput") && (v < 0 || v > 15) {
			return fmt.Errorf("invalid jukebox comparator output")
		}
		if path == "/damagePerBlock" && v < 0 {
			return fmt.Errorf("negative tool damage")
		}
		if path == "/generation" && (v < 0 || v > 3) {
			return fmt.Errorf("invalid book generation")
		}
		if strings.HasSuffix(path, "/amplifier") && (v < 0 || v > 255) {
			return fmt.Errorf("invalid effect amplifier")
		}
	case float32:
		if (strings.HasSuffix(path, "/useDuration") || strings.HasSuffix(path, "/lengthInSeconds") || strings.HasSuffix(path, "/diameter") || (id == dc.ComponentInstrument && path == "/range")) && v <= 0 {
			return fmt.Errorf("component value must be positive")
		}
		if strings.HasSuffix(path, "/probability") && (v < 0 || v > 1) {
			return fmt.Errorf("invalid effect probability")
		}
		if (path == "/consume_seconds" || path == "/defaultMiningSpeed" || path == "/diameter") && v < 0 {
			return fmt.Errorf("negative component value")
		}
	case string:
		if id == dc.ComponentWrittenBookContent && (path == "/rawTitle" || path == "/filteredTitle") && len(utf16.Encode([]rune(v))) > 32 {
			return fmt.Errorf("book title too long")
		}
		if id == dc.ComponentWritableBookContent && (path == "/pages/entry/content" || path == "/pages/entry/filteredContent") && len(utf16.Encode([]rune(v))) > 1024 {
			return fmt.Errorf("book page too long")
		}
	}
	return nil
}
