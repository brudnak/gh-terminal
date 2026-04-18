package ticker

import (
	"strings"
	"testing"
)

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		name  string
		value *float64
		want  string
	}{
		{name: "nil", value: nil, want: "N/A"},
		{name: "large", value: floatPtr(4854.6), want: "$ 4,854.60"},
		{name: "mid", value: floatPtr(175.5), want: "$ 175.50"},
		{name: "small trim", value: floatPtr(0.25), want: "$ 0.25"},
		{name: "small fixed", value: floatPtr(0.011), want: "$ 0.0110"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatPrice(tt.value); got != tt.want {
				t.Fatalf("formatPrice() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatChange(t *testing.T) {
	if got, _ := formatChange(floatPtr(2.83)); got != "▲ +2.83%" {
		t.Fatalf("positive format mismatch: %s", got)
	}

	if got, _ := formatChange(floatPtr(-1.2)); got != "▼ -1.20%" {
		t.Fatalf("negative format mismatch: %s", got)
	}

	if got, _ := formatChange(nil); got != "N/A" {
		t.Fatalf("nil format mismatch: %s", got)
	}
}

func TestRenderSVGIncludesStatusAndRows(t *testing.T) {
	view := tickerView{
		Assets: []renderedAsset{
			{Symbol: "GOLD", PriceText: "$ 4,854.60", ChangeText: "▲ +2.20%", ChangeFill: "__positive__"},
			{Symbol: "BTC", PriceText: "$ 77,285.42", ChangeText: "▲ +2.83%", ChangeFill: "__positive__"},
			{Symbol: "TSLA", PriceText: "$ 175.50", ChangeText: "▼ -1.20%", ChangeFill: "__negative__"},
			{Symbol: "VTI", PriceText: "$ 280.15", ChangeText: "▲ +0.80%", ChangeFill: "__positive__"},
		},
		StatusLine:      "# source status: live snapshot",
		StatusLineFill:  "__muted__",
		RenderedAtLabel: "2026-04-17 12:30 UTC",
	}
	svg := RenderSVG(view, DarkTheme)

	for _, symbol := range []string{"GOLD", "BTC", "TSLA", "VTI"} {
		if !strings.Contains(svg, symbol) {
			t.Fatalf("rendered svg missing symbol %q", symbol)
		}
	}

	if !strings.Contains(svg, "terminal-ticker") {
		t.Fatal("rendered svg missing terminal-ticker header")
	}

	if !strings.Contains(svg, "# source status: live snapshot") {
		t.Fatal("rendered svg missing status line")
	}
}

func TestLightThemeUsesLightPalette(t *testing.T) {
	view := tickerView{
		Assets: []renderedAsset{
			{Symbol: "BTC", PriceText: "$ 77,285.42", ChangeText: "▲ +2.83%", ChangeFill: "__positive__"},
		},
		StatusLine:      "# source status: live snapshot",
		StatusLineFill:  "__muted__",
		RenderedAtLabel: "2026-04-17 12:30 UTC",
	}

	svg := RenderSVG(view, LightTheme)
	if !strings.Contains(svg, `fill="#FFFFFF"`) {
		t.Fatal("light theme missing white background")
	}
	if !strings.Contains(svg, `fill="#1A7F37"`) {
		t.Fatal("light theme missing green accent")
	}
}

func floatPtr(value float64) *float64 {
	return &value
}
