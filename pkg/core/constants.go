package core

// ScrollStrategy defines how a scrollable container is identified.
type ScrollStrategy string

const (
	// ScrollStrategyWindow targets the main window viewport.
	ScrollStrategyWindow ScrollStrategy = ""
	// ScrollStrategyGenericList targets a common list/dropdown container (generic listbox, dropdown, etc.).
	ScrollStrategyGenericList ScrollStrategy = "generic-list"
)
