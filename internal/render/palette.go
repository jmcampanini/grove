package render

import "charm.land/lipgloss/v2"

var (
	// ColorGray is the muted text color.
	ColorGray = lipgloss.Color("245")
	// ColorGreen is the success color.
	ColorGreen = lipgloss.Color("76")
	// ColorRed is the failure color.
	ColorRed = lipgloss.Color("196")
	// ColorYellow is the pending color.
	ColorYellow = lipgloss.Color("214")
)

const (
	// IconCheck is the success glyph.
	IconCheck = "\ueab2" // nf-cod-pass
	// IconCross is the failure glyph.
	IconCross = "\ueab8" // nf-cod-error
	// IconPending is the pending-state glyph.
	IconPending = "\uf017" // nf-fa-clock
)
