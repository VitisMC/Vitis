package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// TabConfig holds tab list header/footer templates.
type TabConfig struct {
	Header string `yaml:"header"`
	Footer string `yaml:"footer"`
}

// DefaultTabConfig returns default tab configuration.
func DefaultTabConfig() TabConfig {
	return TabConfig{
		Header: "§6§lVitis Server\n§7Welcome!",
		Footer: "§7Игроки: §f{online}§7/§f{max} §8| §7TPS: §a{tps}\n§7CPU: §f{cpu}% §8| §7RAM: §f{ram_mib} MiB\n§7Генерация: §f{generated_per_sec}/с §8| §7Отправка: §f{sent_per_sec}/с\n§7Генерация чанка: §f{chunk_avg_ms} мс ср. §8| §f{chunk_last_ms} мс посл.\n§7Полный путь: §f{full_path_per_sec}/с §8| §f{full_path_avg_ms} мс ср. §8| §f{full_path_last_ms} мс посл.",
	}
}

// LoadTabConfig reads tab.yaml from the given path, falling back to defaults.
func LoadTabConfig(path string) TabConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultTabConfig()
	}
	var cfg TabConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultTabConfig()
	}
	if cfg.Header == "" && cfg.Footer == "" {
		return DefaultTabConfig()
	}
	return cfg
}

// RenderHeader renders the header template with placeholder values.
func (t TabConfig) RenderHeader(online, max int, tps float64) string {
	header, _ := t.RenderLive(TabStats{Online: online, Max: max, TPS: tps})
	return header
}

// RenderFooter renders the footer template with placeholder values.
func (t TabConfig) RenderFooter(online, max int, tps float64) string {
	_, footer := t.RenderLive(TabStats{Online: online, Max: max, TPS: tps})
	return footer
}

// TabStats contains live values available to header and footer templates.
type TabStats struct {
	Online, Max                                                           int
	TPS, CPUPercent, GeneratedPerSec, SentPerSec, ChunkAvgMS, ChunkLastMS float64
	RAMBytes                                                              uint64
	FullPathPerSec, FullPathAvgMS, FullPathLastMS                         float64
}

// RenderLive renders both tab components with current server metrics.
func (t TabConfig) RenderLive(stats TabStats) (string, string) {
	render := func(template string) string {
		r := strings.NewReplacer(
			"{online}", strconv.Itoa(stats.Online),
			"{max}", strconv.Itoa(stats.Max),
			"{server}", "Vitis",
			"{tps}", fmt.Sprintf("%.1f", stats.TPS),
			"{cpu}", fmt.Sprintf("%.0f", stats.CPUPercent),
			"{ram_mib}", fmt.Sprintf("%.0f", float64(stats.RAMBytes)/1048576),
			"{generated_per_sec}", fmt.Sprintf("%.1f", stats.GeneratedPerSec),
			"{sent_per_sec}", fmt.Sprintf("%.1f", stats.SentPerSec),
			"{chunk_avg_ms}", fmt.Sprintf("%.1f", stats.ChunkAvgMS),
			"{chunk_last_ms}", fmt.Sprintf("%.1f", stats.ChunkLastMS),
			"{full_path_per_sec}", fmt.Sprintf("%.1f", stats.FullPathPerSec),
			"{full_path_avg_ms}", fmt.Sprintf("%.1f", stats.FullPathAvgMS),
			"{full_path_last_ms}", fmt.Sprintf("%.1f", stats.FullPathLastMS),
			"\\n", "\n",
		)
		return r.Replace(template)
	}
	return render(t.Header), render(t.Footer)
}
