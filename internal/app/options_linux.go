//go:build linux

package app

import "github.com/wailsapp/wails/v2/pkg/options/linux"

// platformLinuxOptions returns the Linux (GTK3 + WebKit2GTK) window options.
//
// WebviewGpuPolicyNever matches the Wails v2 default (see
// github.com/wailsapp/wails#2977). Disabling hardware acceleration keeps the
// window paintable on the messy mix of NVIDIA proprietary drivers, Wayland
// compositors without render-node access and older Mesa stacks that ship with
// current LTS distributions. Daygo is a settings/timeline UI, not a 3D app,
// so the CPU path is acceptable for the launch posture.
//
// WindowIsTranslucent is left false on purpose: the platform's compositor
// hint for translucent windows only matters once a real theme asks for
// translucency, and the frontend would otherwise show a checkerboard when
// the GTK theme ships without compositor support.
func platformLinuxOptions() *linux.Options {
	return &linux.Options{
		ProgramName:      "Daygo",
		WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		Messages:         linux.DefaultMessages(),
	}
}
