package app

import (
	"fmt"

	"github.com/Jwz-git/Daygo/frontend"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

// Run starts the Daygo desktop shell.
func Run() error {
	err := wails.Run(&options.App{
		Title:     "Daygo",
		Width:     1180,
		Height:    760,
		MinWidth:  880,
		MinHeight: 600,
		/*
		 * Not frameless: a frameless NSWindow drops the standard window frame,
		 * and with it both the traffic-light controls and the native rounded
		 * window corners. Hiding the titlebar instead (mac.TitleBarHiddenInset)
		 * keeps content edge-to-edge while leaving those two to the OS.
		 */
		CSSDragProperty: "--wails-draggable",
		CSSDragValue:    "drag",
		// Shown only between window creation and the first webview paint. The
		// light palette base is the least jarring default: macOS defaults to a
		// light system appearance, and the app follows it until the user says
		// otherwise.
		BackgroundColour: &options.RGBA{R: 233, G: 240, B: 251, A: 1},
		AssetServer: &assetserver.Options{
			Assets: frontend.Assets,
		},
		Mac: &mac.Options{
			/*
			 * TitleBarHidden (not TitleBarHiddenInset): both keep the native
			 * traffic lights, but HiddenInset also attaches an empty NSToolbar,
			 * and on macOS 26 a window with a toolbar is drawn with a noticeably
			 * larger corner radius than a titlebar-only window. Dropping the
			 * toolbar is what keeps the window corners tight.
			 */
			TitleBar:             mac.TitleBarHidden(),
			WebviewIsTransparent: false,
		},
	})
	if err != nil {
		return fmt.Errorf("run Daygo desktop shell: %w", err)
	}
	return nil
}
