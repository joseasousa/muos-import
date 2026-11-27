package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type uiScreen int

const (
	uiHome uiScreen = iota
	uiConfig
	uiBrowse
	uiSave
	uiSync
	uiRunning
	uiResult
)

type uiRow struct {
	label, detail string
}

type handheld struct {
	path, side string
	screen     uiScreen
	cursor     int
	editor     configEditor
	field      int
	dir        string
	folders    []string
	result     string
	success    bool
	backTo     uiScreen
	job        <-chan error
}

func newHandheld(path, side string) *handheld {
	h := &handheld{path: path, side: side}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		h.edit()
	}
	return h
}

func (h *handheld) show(screen uiScreen) {
	h.screen, h.cursor = screen, 0
}

func (h *handheld) report(err error, success string, backTo uiScreen) {
	h.success = err == nil
	h.result, h.backTo = success, backTo
	if err != nil {
		h.result = err.Error()
	}
	h.show(uiResult)
}

func (h *handheld) edit() {
	editor, err := openConfigEditor(h.path, h.side)
	if err != nil {
		h.report(err, "", uiHome)
		return
	}
	editor.config.Side = h.side
	if editor.config.Cores == nil {
		editor.config.Cores = map[string]Roots{}
	}
	h.editor = editor
	h.show(uiConfig)
}

func (h *handheld) fields() []uiRow {
	c := h.editor.config
	return []uiRow{
		{"SD2 card", c.Card},
		{"ROM library", c.ROMs},
		{"Gambatte saves", c.Cores["Gambatte"].Saves},
		{"mGBA saves", c.Cores["mGBA"].Saves},
	}
}

func (h *handheld) chooseFolder(path string) {
	switch h.field {
	case 0:
		h.editor.config.Card = path
	case 1:
		h.editor.config.ROMs = path
	default:
		core := supportedCores[h.field-2]
		roots := h.editor.config.Cores[core]
		roots.Saves = path
		h.editor.config.Cores[core] = roots
	}
	h.show(uiConfig)
	h.cursor = h.field
}

func (h *handheld) browse(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		h.report(err, "", h.screen)
		return
	}
	h.folders = h.folders[:0]
	for _, entry := range entries {
		if entry.IsDir() {
			h.folders = append(h.folders, entry.Name())
		} else if entry.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(filepath.Join(dir, entry.Name()))
			if err == nil && info.IsDir() {
				h.folders = append(h.folders, entry.Name())
			}
		}
	}
	h.dir = dir
	h.show(uiBrowse)
}

const browseUpLabel = "Go up one level"

func (h *handheld) browseActions() []uiRow {
	rows := []uiRow{{label: "Select this folder"}}
	if h.dir != "/" {
		rows = append(rows, uiRow{label: browseUpLabel})
	}
	if h.field >= 2 {
		rows = append(rows, uiRow{label: "Leave unset"})
	}
	return rows
}

func (h *handheld) view() (string, string, []uiRow) {
	switch h.screen {
	case uiConfig:
		rows := h.fields()
		for i := range rows {
			if rows[i].detail == "" {
				rows[i].detail = "No folder selected"
			}
		}
		return "Configure", h.side, append(rows, uiRow{label: "Save configuration"})
	case uiBrowse:
		rows := h.browseActions()
		for _, folder := range h.folders {
			rows = append(rows, uiRow{label: folder})
		}
		return h.fields()[h.field].label, h.dir, rows
	case uiSave:
		return "Save configuration?", "", []uiRow{{label: "Back"}, {label: "Save"}}
	case uiSync:
		return "Start sync?", "Close RetroArch before continuing.", []uiRow{{label: "Back"}, {label: "Sync"}}
	case uiRunning:
		return "Syncing", "Wait for the sync to finish.", nil
	case uiResult:
		title := "Done"
		if !h.success {
			title = "Action failed"
		}
		return title, h.result, nil
	default:
		return "SaveSync", h.side, []uiRow{
			{label: "Configure"},
			{label: "Check configuration"},
			{label: "Sync"},
			{label: "Exit"},
		}
	}
}

func (h *handheld) check() error {
	var c Config
	if err := readJSON(h.path, &c); err != nil {
		return fmt.Errorf("configuration (%s): %w", h.path, err)
	}
	if c.Side != h.side {
		return fmt.Errorf("Side: configuration uses %q and the launcher uses %q. Open Configure to fix this", c.Side, h.side)
	}
	return validateConfig(c, true)
}

func (h *handheld) back() bool {
	switch h.screen {
	case uiHome:
		return true
	case uiConfig, uiSync:
		h.show(uiHome)
	case uiSave:
		h.show(uiConfig)
		h.cursor = len(h.fields())
	case uiBrowse:
		h.show(uiConfig)
		h.cursor = h.field
	case uiResult:
		h.show(h.backTo)
	}
	return false
}

func (h *handheld) activate() bool {
	switch h.screen {
	case uiHome:
		switch h.cursor {
		case 0:
			h.edit()
		case 1:
			h.report(h.check(), "Configuration is valid", uiHome)
		case 2:
			if err := h.check(); err != nil {
				h.report(err, "", uiHome)
			} else {
				h.show(uiSync)
			}
		case 3:
			return true
		}
	case uiConfig:
		fields := h.fields()
		if h.cursor == len(fields) {
			h.show(uiSave)
			break
		}
		h.field = h.cursor
		start := fields[h.field].detail
		info, err := os.Stat(start)
		if !filepath.IsAbs(start) || err != nil || !info.IsDir() {
			start = "/"
		}
		h.browse(start)
	case uiBrowse:
		switch label := h.browseActions(); {
		case h.cursor >= len(label):
			h.browse(filepath.Join(h.dir, h.folders[h.cursor-len(label)]))
		case h.cursor == 0:
			h.chooseFolder(h.dir)
		case label[h.cursor].label == browseUpLabel:
			h.browse(filepath.Dir(h.dir))
		default:
			h.chooseFolder("")
		}
	case uiSave:
		if h.cursor == 0 {
			return h.back()
		}
		err := h.editor.save(h.path)
		back := uiHome
		if err != nil {
			back = uiConfig
		}
		h.report(err, "Configuration saved", back)
	case uiSync:
		if h.cursor == 0 {
			return h.back()
		}
		if err := h.check(); err != nil {
			h.report(err, "", uiHome)
			break
		}
		job := make(chan error, 1)
		h.job = job
		h.show(uiRunning)
		go func() { job <- syncToLog(h.path, h.side) }()
	case uiResult:
		return h.back()
	}
	return false
}

func syncToLog(config, side string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := filepath.Join(filepath.Dir(config), "sync.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer log.Close()
	if _, err := fmt.Fprintln(log, "\n--- Sync ---"); err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd := exec.Command(exe, "-config", config, "-action", "sync", "-side", side)
	cmd.Stdout, cmd.Stderr = log, io.MultiWriter(log, &stderr)
	if err := cmd.Run(); err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return errors.New(message)
		}
		fmt.Fprintln(log, err)
		return err
	}
	return log.Sync()
}
