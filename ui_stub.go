//go:build !sdl || !cgo

package main

import "errors"

func launchUI(_, _ string) error {
	return errors.New("SDL2 interface is unavailable in this binary")
}
