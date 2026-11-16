// Package dialogs exposes native operating-system dialogs to the interface.
package dialogs

import (
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const defaultFolderTitle = "Choose a folder"

// Service opens native dialogs on behalf of the interface.
type Service struct{}

// NewService returns a dialogs service.
func NewService() *Service {
	return &Service{}
}

// ChooseFolder asks the operator to pick an existing folder and returns its
// path. An empty path means the dialog was cancelled.
func (s *Service) ChooseFolder(title string) (string, error) {
	app := application.Get()

	dialog := app.Dialog.OpenFile().
		SetTitle(folderTitle(title)).
		CanChooseDirectories(true).
		CanChooseFiles(false)
	if window := app.Window.Current(); window != nil {
		dialog.AttachToWindow(window)
	}

	return dialog.PromptForSingleSelection()
}

func folderTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return defaultFolderTitle
	}
	return title
}
