//go:build !sdl || !cgo

package main

import "errors"

func launchUI(_, _ string) error {
	return errors.New("interface SDL2 indisponível neste binário")
}
