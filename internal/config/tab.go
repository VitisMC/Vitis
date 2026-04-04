package config

import (
	"fmt"
	"os"
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
		Footer: "§7Players: §f{online}§7/§f{max} §8| §7TPS: §a{tps}",
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
	return renderTemplate(t.Header, online, max, tps)
}

// RenderFooter renders the footer template with placeholder values.
func (t TabConfig) RenderFooter(online, max int, tps float64) string {
	return renderTemplate(t.Footer, online, max, tps)
}

func renderTemplate(tmpl string, online, max int, tps float64) string {
	r := strings.NewReplacer(
		"{online}", fmt.Sprintf("%d", online),
		"{max}", fmt.Sprintf("%d", max),
		"{server}", "Vitis",
		"{tps}", fmt.Sprintf("%.1f", tps),
		"\\n", "\n",
	)
	return r.Replace(tmpl)
}
