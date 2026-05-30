package cmd

import (
	"image/color"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

func lightDarkColor(hasDarkBackground bool, light, dark string) color.Color {
	return lipgloss.LightDark(hasDarkBackground)(lipgloss.Color(light), lipgloss.Color(dark))
}

func detectDarkBackgroundFromEnv() (dark bool, ok bool) {
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
