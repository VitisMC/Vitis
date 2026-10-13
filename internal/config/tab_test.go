package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderLiveTab verifies all live placeholders and line separators.
func TestRenderLiveTab(t *testing.T) {
	tab := TabConfig{Header: "{server} {online}/{max}", Footer: "{tps} {cpu} {ram_mib} {generated_per_sec} {sent_per_sec} {chunk_avg_ms} {chunk_last_ms} {full_path_per_sec} {full_path_avg_ms} {full_path_last_ms}\\nOK"}
	header, footer := tab.RenderLive(TabStats{Online: 2, Max: 10, TPS: 19.9, CPUPercent: 158, RAMBytes: 128 << 20, GeneratedPerSec: 205.2, SentPerSec: 98.3, ChunkAvgMS: 4.8, ChunkLastMS: 5.1, FullPathPerSec: 91.2, FullPathAvgMS: 34.5, FullPathLastMS: 21.7})
	if header != "Vitis 2/10" || footer != "19.9 158 128 205.2 98.3 4.8 5.1 91.2 34.5 21.7\nOK" {
		t.Fatalf("unexpected tab content: %q / %q", header, footer)
	}
	if strings.Contains(footer, "{") {
		t.Fatalf("unresolved placeholder in %q", footer)
	}
}

// TestShippedTabConfigRenders verifies the custom template resolves its placeholders.
func TestShippedTabConfigRenders(t *testing.T) {
	tab := LoadTabConfig(filepath.Join("..", "..", "configs", "tab.yaml"))
	header, footer := tab.RenderLive(TabStats{FullPathPerSec: 12.5, FullPathAvgMS: 34.5, FullPathLastMS: 28.1})
	if strings.Contains(header+footer, "{") || header+footer == "" {
		t.Fatalf("invalid rendered custom tab: %q", footer)
	}
}
