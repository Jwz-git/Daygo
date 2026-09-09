package app

import (
	"fmt"

	"github.com/Jwz-git/Daygo/frontend"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// Run starts the Daygo desktop shell.
func Run() error {
	err := wails.Run(&options.App{
		Title:     "Daygo",
		Width:     1040,
		Height:    680,
		MinWidth:  760,
		MinHeight: 520,
		AssetServer: &assetserver.Options{
			Assets: frontend.Assets,
		},
		BackgroundColour: &options.RGBA{R: 239, G: 242, B: 246, A: 1},
	})
	if err != nil {
		return fmt.Errorf("run Daygo desktop shell: %w", err)
	}
	return nil
}
