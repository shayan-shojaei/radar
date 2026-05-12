package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/shayan-shojaei/radar/internal/specsstore"
	"github.com/shayan-shojaei/radar/internal/tui/views"
)

// RunSpecPicker launches a short-lived TUI for selecting or adding a spec.
// Returns the chosen URL, or "" if the user quit without selecting.
func RunSpecPicker(saved, recent []specsstore.SavedSpec, storageDir string) (string, error) {
	m := views.NewSpecPickerModel(saved, recent, storageDir)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}
	if picker, ok := finalModel.(views.SpecPickerModel); ok {
		return picker.Chosen(), nil
	}
	return "", nil
}
