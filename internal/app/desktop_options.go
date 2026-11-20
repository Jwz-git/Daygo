//go:build daygo_desktop

package app

import (
	"runtime"

	"github.com/Jwz-git/Daygo/internal/platform"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func desktopWindowOptions(goos string) application.WebviewWindowOptions {
	opts := application.WebviewWindowOptions{
		Name: "main", Title: "Daygo", Width: 1180, Height: 760,
		MinWidth: 880, MinHeight: 600,
		DefaultContextMenuDisabled: true,
		BackgroundColour:           application.NewRGBA(233, 240, 251, 255),
	}
	switch goos {
	case "darwin":
		opts.Mac.TitleBar = application.MacTitleBarHidden
	case "windows":
		opts.Frameless = true
		opts.Windows = application.WindowsWindow{Theme: application.SystemDefault, BackdropType: application.Mica, DisableIcon: true}
	case "linux":
		opts.Linux.WebviewGpuPolicy = application.WebviewGpuPolicyNever
	}
	return opts
}

type wailsApplicationPicker struct {
	host   *application.App
	window application.Window
	labels func() NativeUiLabelsDTO
}

func (picker wailsApplicationPicker) PickApplication() (string, error) {
	opts := applicationPickerOptions(runtime.GOOS, picker.labels())
	dialog := picker.host.Dialog.OpenFile().
		CanChooseFiles(true).CanChooseDirectories(false).
		ResolvesAliases(true).TreatsFilePackagesAsDirectories(false).
		SetTitle(opts.Title).SetDirectory(opts.DefaultDirectory).
		AttachToWindow(picker.window)
	for _, filter := range opts.Filters {
		dialog.AddFilter(filter.DisplayName, filter.Pattern)
	}
	return dialog.PromptForSingleSelection()
}

// Keep the existing translated menu surface; v3's default File/View/Help
// menus would add English-only actions that Daygo does not expose.
func installDesktopMenu(desktop *application.App, labels platform.ApplicationMenuLabels) {
	if runtime.GOOS != "darwin" {
		return
	}

	if menu := desktop.Menu.GetApplicationMenu(); menu != nil {
		// Reuse menu objects across language changes; replacing them leaves the
		// framework's menu-item registrations alive for the process lifetime.
		for role, label := range map[application.Role]string{
			application.Hide: labels.Hide, application.HideOthers: labels.HideOthers,
			application.ShowAll: labels.ShowAll, application.Quit: labels.Quit,
			application.EditMenu: labels.Edit, application.Undo: labels.Undo, application.Redo: labels.Redo,
			application.Cut: labels.Cut, application.Copy: labels.Copy, application.Paste: labels.Paste,
			application.PasteAndMatchStyle: labels.PasteMatch, application.Delete: labels.Delete,
			application.SelectAll: labels.SelectAll, application.SpeechMenu: labels.Speech,
			application.StartSpeaking: labels.StartSpeaking, application.StopSpeaking: labels.StopSpeaking,
			application.WindowMenu: labels.Window, application.Minimise: labels.Minimize,
			application.Zoom: labels.Zoom, application.ToggleFullscreen: labels.FullScreen,
		} {
			if item := menu.FindByRole(role); item != nil {
				item.SetLabel(label)
			}
		}
		return
	}
	menu := application.NewMenu()
	add := func(parent *application.Menu, role application.Role, label string) {
		parent.AddRole(role)
		if item := parent.FindByRole(role); item != nil {
			item.SetLabel(label)
		}
	}
	appMenu := menu.AddSubmenu("Daygo")
	add(appMenu, application.Hide, labels.Hide)
	add(appMenu, application.HideOthers, labels.HideOthers)
	add(appMenu, application.ShowAll, labels.ShowAll)
	appMenu.AddSeparator()
	add(appMenu, application.Quit, labels.Quit)
	edit := menu.AddSubmenu(labels.Edit)
	menu.FindByLabel(labels.Edit).SetRole(application.EditMenu)
	add(edit, application.Undo, labels.Undo)
	add(edit, application.Redo, labels.Redo)
	edit.AddSeparator()
	add(edit, application.Cut, labels.Cut)
	add(edit, application.Copy, labels.Copy)
	add(edit, application.Paste, labels.Paste)
	add(edit, application.PasteAndMatchStyle, labels.PasteMatch)
	add(edit, application.Delete, labels.Delete)
	add(edit, application.SelectAll, labels.SelectAll)
	speech := edit.AddSubmenu(labels.Speech)
	edit.FindByLabel(labels.Speech).SetRole(application.SpeechMenu)
	add(speech, application.StartSpeaking, labels.StartSpeaking)
	add(speech, application.StopSpeaking, labels.StopSpeaking)
	windows := menu.AddSubmenu(labels.Window)
	menu.FindByLabel(labels.Window).SetRole(application.WindowMenu)
	add(windows, application.Minimise, labels.Minimize)
	add(windows, application.Zoom, labels.Zoom)
	add(windows, application.ToggleFullscreen, labels.FullScreen)
	desktop.Menu.SetApplicationMenu(menu)
}
