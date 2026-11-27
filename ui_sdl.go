//go:build sdl && cgo

package main

/*
#cgo pkg-config: sdl2
#define SDL_MAIN_HANDLED
#include <SDL.h>
#include <stdlib.h>

enum { UI_NONE, UI_UP, UI_DOWN, UI_ACCEPT, UI_BACK, UI_CLOSE, UI_REDRAW };

static void ui_guid(int index, char *buffer, int size) {
	SDL_JoystickGetGUIDString(SDL_JoystickGetDeviceGUID(index), buffer, size);
}

static int ui_event(int *index) {
	SDL_Event event;
	if (!SDL_WaitEventTimeout(&event, 80)) return UI_NONE;

	switch (event.type) {
	case SDL_QUIT:
		return UI_CLOSE;
	case SDL_WINDOWEVENT:
		if (event.window.event == SDL_WINDOWEVENT_EXPOSED ||
			event.window.event == SDL_WINDOWEVENT_SIZE_CHANGED) return UI_REDRAW;
		return UI_NONE;
	case SDL_JOYDEVICEADDED:
		*index = event.jdevice.which;
		return UI_NONE;
	case SDL_KEYDOWN:
		switch (event.key.keysym.sym) {
		case SDLK_UP: return UI_UP;
		case SDLK_DOWN: return UI_DOWN;
		case SDLK_RETURN:
		case SDLK_KP_ENTER: return event.key.repeat ? UI_NONE : UI_ACCEPT;
		case SDLK_ESCAPE:
		case SDLK_BACKSPACE: return event.key.repeat ? UI_NONE : UI_BACK;
		}
		break;
	case SDL_CONTROLLERBUTTONDOWN:
		switch (event.cbutton.button) {
		case SDL_CONTROLLER_BUTTON_DPAD_UP: return UI_UP;
		case SDL_CONTROLLER_BUTTON_DPAD_DOWN: return UI_DOWN;
		case SDL_CONTROLLER_BUTTON_A: return UI_ACCEPT;
		case SDL_CONTROLLER_BUTTON_B: return UI_BACK;
		}
		break;
	}
	return UI_NONE;
}
*/
import "C"

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	screenWidth  = 640
	screenHeight = 480
	uiMargin     = 30
)

func init() {
	// SDL video operations must run on the process's initial OS thread.
	runtime.LockOSThread()
}

func sdlError(operation string) error {
	return fmt.Errorf("SDL2: %s: %s", operation, C.GoString(C.SDL_GetError()))
}

func launchUI(path, side string) error {
	if !validSide(side) {
		return fmt.Errorf("Side: invalid firmware (%q)", side)
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	C.SDL_SetMainReady()
	if C.SDL_Init(C.SDL_INIT_VIDEO|C.SDL_INIT_GAMECONTROLLER) != 0 {
		return sdlError("initialize")
	}
	defer C.SDL_Quit()

	title := C.CString("SaveSync")
	defer C.free(unsafe.Pointer(title))
	window := C.SDL_CreateWindow(title, C.SDL_WINDOWPOS_CENTERED, C.SDL_WINDOWPOS_CENTERED,
		screenWidth, screenHeight, C.SDL_WINDOW_FULLSCREEN_DESKTOP)
	if window == nil {
		return sdlError("open window")
	}
	defer C.SDL_DestroyWindow(window)

	renderer := C.SDL_CreateRenderer(window, -1, 0)
	if renderer == nil {
		return sdlError("create renderer")
	}
	defer C.SDL_DestroyRenderer(renderer)
	if C.SDL_RenderSetLogicalSize(renderer, screenWidth, screenHeight) != 0 {
		return sdlError("set drawing area")
	}
	texture := C.SDL_CreateTexture(renderer, C.SDL_PIXELFORMAT_RGBA32,
		C.SDL_TEXTUREACCESS_STREAMING, screenWidth, screenHeight)
	if texture == nil {
		return sdlError("create texture")
	}
	defer C.SDL_DestroyTexture(texture)

	canvas, err := newUICanvas()
	if err != nil {
		return err
	}
	defer canvas.close()
	var controller *C.SDL_GameController
	// SDL queues joystick-added events even if its database lacks a mapping.
	defer func() {
		if controller != nil {
			C.SDL_GameControllerClose(controller)
		}
	}()

	h := newHandheld(path, side)
	dirty, closing := true, false
	for {
		if dirty {
			canvas.paint(h)
			if C.SDL_UpdateTexture(texture, nil, unsafe.Pointer(&canvas.Pix[0]), C.int(canvas.Stride)) != 0 ||
				C.SDL_RenderClear(renderer) != 0 ||
				C.SDL_RenderCopy(renderer, texture, nil, nil) != 0 {
				if h.job != nil {
					<-h.job
				}
				return sdlError("draw interface")
			}
			C.SDL_RenderPresent(renderer)
			dirty = false
		}
		index := C.int(-1)
		event := C.ui_event(&index)
		if controller != nil && C.SDL_GameControllerGetAttached(controller) == C.SDL_FALSE {
			C.SDL_GameControllerClose(controller)
			controller = nil
		}
		if controller == nil && index >= 0 {
			configureController(index, side)
			controller = C.SDL_GameControllerOpen(index)
		}
		if h.job != nil {
			closing = closing || event == C.UI_CLOSE
			dirty = dirty || event == C.UI_REDRAW
			select {
			case err := <-h.job:
				h.job = nil
				if closing {
					return err
				}
				h.report(err, "Sync complete. See sync.log for details.", uiHome)
				dirty = true
			default:
			}
			continue
		}
		switch event {
		case C.UI_CLOSE:
			return nil
		case C.UI_REDRAW:
		case C.UI_BACK:
			if h.back() {
				return nil
			}
		case C.UI_ACCEPT:
			if h.activate() {
				return nil
			}
		case C.UI_UP:
			if h.cursor > 0 {
				h.cursor--
			}
		case C.UI_DOWN:
			_, _, rows := h.view()
			if h.cursor+1 < len(rows) {
				h.cursor++
			}
		default:
			continue
		}
		dirty = true
	}
}

func configureController(index C.int, side string) {
	if os.Getenv("SDL_GAMECONTROLLERCONFIG") != "" || os.Getenv("SDL_GAMECONTROLLERCONFIG_FILE") != "" {
		return
	}
	name := C.GoString(C.SDL_JoystickNameForIndex(index))
	var buffer [33]C.char
	C.ui_guid(index, &buffer[0], C.int(len(buffer)))
	guid := C.GoString(&buffer[0])
	var mapping string
	if side == "knulli" {
		for _, path := range []string{
			"/userdata/system/configs/emulationstation/es_input.cfg",
			"/usr/share/emulationstation/es_input.cfg",
		} {
			if mapping = stationMapping(path, guid, name); mapping != "" {
				break
			}
		}
	}
	if mapping == "" && name == "muOS-Keys" {
		mapping = guid + ",muOS-Keys,a:b0,b:b1,dpup:h0.1,dpdown:h0.4,platform:Linux,"
	}
	if mapping != "" {
		value := C.CString(mapping)
		defer C.free(unsafe.Pointer(value))
		if C.SDL_GameControllerAddMapping(value) < 0 {
			fmt.Fprintln(os.Stderr, sdlError("map controller"))
		}
	}
}

type uiCanvas struct {
	*image.RGBA
	title, body, small font.Face
}

func newUICanvas() (*uiCanvas, error) {
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	canvas := &uiCanvas{RGBA: image.NewRGBA(image.Rect(0, 0, screenWidth, screenHeight))}
	for _, spec := range []struct {
		target *font.Face
		size   float64
	}{{&canvas.title, 34}, {&canvas.body, 23}, {&canvas.small, 17}} {
		*spec.target, err = opentype.NewFace(parsed, &opentype.FaceOptions{
			Size: spec.size, DPI: 72, Hinting: font.HintingFull,
		})
		if err != nil {
			canvas.close()
			return nil, err
		}
	}
	return canvas, nil
}

func (c *uiCanvas) close() {
	for _, face := range []font.Face{c.title, c.body, c.small} {
		if face != nil {
			face.Close()
		}
	}
}

var (
	uiBackground = image.NewUniform(color.RGBA{15, 24, 31, 255})
	uiSurface    = image.NewUniform(color.RGBA{24, 38, 47, 255})
	uiHighlight  = image.NewUniform(color.RGBA{28, 76, 68, 255})
	uiAccent     = image.NewUniform(color.RGBA{114, 237, 189, 255})
	uiText       = image.NewUniform(color.RGBA{241, 246, 248, 255})
	uiMuted      = image.NewUniform(color.RGBA{180, 201, 211, 255})
	uiError      = image.NewUniform(color.RGBA{255, 177, 159, 255})
)

func (c *uiCanvas) fill(x, y, w, h int, pigment *image.Uniform) {
	draw.Draw(c.RGBA, image.Rect(x, y, x+w, y+h), pigment, image.Point{}, draw.Src)
}

func (c *uiCanvas) text(face font.Face, pigment *image.Uniform, x, y int, text string) {
	d := font.Drawer{Dst: c.RGBA, Src: pigment, Face: face, Dot: fixed.P(x, y)}
	d.DrawString(text)
}

func shorten(face font.Face, text string, width int, keepEnd bool) string {
	if font.MeasureString(face, text).Ceil() <= width {
		return text
	}
	chars := []rune(text)
	for len(chars) > 0 {
		if keepEnd {
			chars = chars[1:]
			text = "…" + string(chars)
		} else {
			chars = chars[:len(chars)-1]
			text = string(chars) + "…"
		}
		if font.MeasureString(face, text).Ceil() <= width {
			return text
		}
	}
	return "…"
}

func wrapUI(face font.Face, text string, width int) []string {
	var lines []string
	var line strings.Builder
	var advance fixed.Int26_6
	for _, r := range text {
		size, _ := face.GlyphAdvance(r)
		if r == '\n' || (line.Len() > 0 && (advance+size).Ceil() > width) {
			lines = append(lines, line.String())
			line.Reset()
			advance = 0
		}
		if r != '\n' {
			line.WriteRune(r)
			advance += size
		}
	}
	if line.Len() > 0 {
		lines = append(lines, line.String())
	}
	return lines
}

func (c *uiCanvas) paint(h *handheld) {
	title, subtitle, rows := h.view()
	c.fill(0, 0, screenWidth, screenHeight, uiBackground)
	c.fill(uiMargin, 25, 34, 4, uiAccent)
	c.text(c.title, uiText, uiMargin, 68, title)
	if len(rows) == 0 {
		pigment := uiMuted
		if h.screen == uiResult && !h.success {
			pigment = uiError
		}
		lines := wrapUI(c.body, subtitle, screenWidth-2*uiMargin)
		for i, line := range lines {
			if i == 11 {
				c.text(c.body, pigment, uiMargin, 110+i*27, "…")
				break
			}
			c.text(c.body, pigment, uiMargin, 110+i*27, line)
		}
	} else {
		c.text(c.small, uiMuted, uiMargin, 97, shorten(c.small, subtitle, screenWidth-2*uiMargin, h.screen == uiBrowse))
		rowHeight := 53
		if h.screen == uiBrowse {
			rowHeight = 40
		}
		visible := 316 / rowHeight
		start := 0
		if h.cursor >= visible {
			start = h.cursor - visible + 1
		}
		for i, row := range rows[start:min(start+visible, len(rows))] {
			y := 114 + i*rowHeight
			background, foreground := uiSurface, uiText
			if start+i == h.cursor {
				background, foreground = uiHighlight, uiAccent
			}
			c.fill(uiMargin, y, screenWidth-2*uiMargin, rowHeight-4, background)
			labelY := y + 28
			if row.detail != "" {
				labelY = y + 22
				c.text(c.small, uiMuted, uiMargin+14, y+42, shorten(c.small, row.detail, screenWidth-2*uiMargin-28, true))
			}
			c.text(c.body, foreground, uiMargin+14, labelY, shorten(c.body, row.label, screenWidth-2*uiMargin-28, false))
		}
	}
	c.fill(uiMargin, 438, screenWidth-2*uiMargin, 1, uiSurface)
	controls := "↑ ↓  Navigate       A  Confirm       B  Back"
	switch h.screen {
	case uiHome:
		controls = "↑ ↓  Navigate       A  Confirm       B  Exit"
	case uiRunning:
		controls = ""
	case uiResult:
		controls = "A / B  Back"
	}
	c.text(c.small, uiMuted, uiMargin, 465, controls)
}
