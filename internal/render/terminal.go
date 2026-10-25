package render

import (
	"image/color"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// LightDarkColor selects a palette color for the terminal background.
func LightDarkColor(hasDarkBackground bool, light, dark string) color.Color {
	return lipgloss.LightDark(hasDarkBackground)(lipgloss.Color(light), lipgloss.Color(dark))
}

// DetectDarkBackgroundFromEnv reads the background color from COLORFGBG when available.
func DetectDarkBackgroundFromEnv() (dark bool, ok bool) {
	parts := strings.Split(os.Getenv("COLORFGBG"), ";")
	if len(parts) < 2 {
		return false, false
	}
	bg, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return false, false
	}
	return bg < 7 || bg == 8, true
}
