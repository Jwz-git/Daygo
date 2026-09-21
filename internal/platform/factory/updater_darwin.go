//go:build darwin && cgo && daygo_updater

package factory

import (
	"log"

	"github.com/Jwz-git/Daygo/internal/platform"
	"github.com/Jwz-git/Daygo/internal/platform/darwin"
)

func NewUpdater() platform.Updater {
	updater, err := darwin.NewUpdater()
	if err != nil {
		log.Printf("platform/factory: Sparkle updater unavailable: %v", err)
		return nil
	}
	return updater
}
