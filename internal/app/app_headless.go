//go:build !daygo_desktop

package app

import "errors"

func Run() error {
	return errors.New("Daygo desktop requires a native build using the desktop build scripts")
}
