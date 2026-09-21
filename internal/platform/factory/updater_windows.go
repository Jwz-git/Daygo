//go:build windows

package factory

import (
	"log"

	"github.com/Jwz-git/Daygo/internal/platform"
	platformwindows "github.com/Jwz-git/Daygo/internal/platform/windows"
)

func NewUpdater() platform.Updater {
	updater, err := platformwindows.NewUpdater()
	if err != nil {
		log.Printf("platform/factory: windows updater unavailable: %v", err)
		return nil
	}
	return updater
}
