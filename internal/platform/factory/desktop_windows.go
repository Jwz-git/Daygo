//go:build windows

package factory

import "github.com/Jwz-git/Daygo/internal/platform/windows"

func PrepareDesktopHost() error { return windows.PrepareDesktopHost() }
