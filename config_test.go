package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckConfiguration(t *testing.T) {
	for _, side := range []string{"muos", "knulli"} {
		t.Run(side, func(t *testing.T) {
			card, roms, saves := t.TempDir(), t.TempDir(), t.TempDir()
			marker := filepath.Join(card, ".savesync-card")
			put(t, marker, "", 100)
			cfg := Config{Card: card, Side: side, ROMs: roms, Cores: map[string]Roots{"mGBA": {Saves: saves}}}
			if err := run(cfg, "check", "", ""); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				field string
				edit  func(*Config)
			}{
				{"Side", func(c *Config) { c.Side = "other" }},
				{"Card", func(c *Config) { c.Card = "relative" }},
				{"ROMs", func(c *Config) { c.ROMs = marker }},
				{"ROMs", func(c *Config) { c.ROMs = filepath.Join(roms, "missing") }},
				{"Cores.mGBA.Saves", func(c *Config) { c.Cores = map[string]Roots{"mGBA": {Saves: "relative"}} }},
				{"Cores.mGBA.States", func(c *Config) { c.Cores = map[string]Roots{"mGBA": {Saves: saves, States: marker}} }},
				{"Cores", func(c *Config) { c.Cores = map[string]Roots{"other": {Saves: saves}} }},
			} {
				bad := cfg
				tc.edit(&bad)
				if err := run(bad, "check", "", ""); err == nil || !strings.Contains(err.Error(), tc.field) {
					t.Errorf("check for %s returned %v", tc.field, err)
				}
			}
			if _, err := os.Stat(filepath.Join(card, "SaveSync")); !os.IsNotExist(err) {
				t.Fatalf("check modified the card: %v", err)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			if err := run(cfg, "check", "", ""); err == nil || !strings.Contains(err.Error(), ".savesync-card") {
				t.Fatalf("check without a marker returned %v", err)
			}
		})
	}
}

func TestHandheldConfiguration(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config.json")
	h := newHandheld(config, "muos")
	if h.screen != uiConfig || h.editor.config.Side != "muos" {
		t.Fatal("first launch did not open muOS configuration")
	}
	h.activate()
	if h.screen != uiBrowse {
		t.Fatal("folder picker did not open")
	}
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	h.browse(child)
	_, _, rows := h.view()
	if len(rows) < 2 || rows[1].label != "Go up one level" {
		t.Fatalf("folder picker did not show the parent action: %+v", rows)
	}
	h.cursor = 1
	h.activate()
	if h.dir != root || h.screen != uiBrowse {
		t.Fatalf("parent action opened %q on screen %v", h.dir, h.screen)
	}
	h.activate()
	if h.editor.config.Card != filepath.Dir(config) || h.screen != uiConfig {
		t.Fatal("selected folder was not applied to the form")
	}
	h.cursor = len(h.fields())
	h.activate()
	if h.screen != uiSave {
		t.Fatal("saving did not prompt for confirmation")
	}
	h.back()
	h.back()
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatalf("cancelled edit wrote the configuration: %v", err)
	}
	h.cursor = 2
	h.activate()
	if h.screen != uiResult || h.success || h.job != nil {
		t.Fatal("sync without configuration did not show the error")
	}
}

func TestHandheldEditPreservesConfiguration(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config.json")
	put(t, config, `{
		"side":"muos","Card":"/sd2","ROMs":"/roms",
		"cores":{"mGBA":{"saves":"/old","States":"/states","FutureKey":true},
		"Other":{"Saves":"/other"}},
		"StateProfile":{"mGBA":"profile"},
		"ApprovedStates":{"hash":true},"BuildFiles":{"/core":"hash"},
		"Future":{"count":9007199254740993}
	}`, 100)
	h := newHandheld(config, "muos")
	h.activate()
	h.field = 3
	h.chooseFolder("/new")
	h.cursor = len(h.fields())
	h.activate()
	h.cursor = 1
	h.activate()
	if h.screen != uiResult || !h.success {
		t.Fatal(h.result)
	}
	var cfg Config
	if err := readJSON(config, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Cores["mGBA"] != (Roots{Saves: "/new", States: "/states"}) ||
		cfg.Cores["Other"].Saves != "/other" || cfg.StateProfile["mGBA"] != "profile" ||
		!cfg.ApprovedStates["hash"] || cfg.BuildFiles["/core"] != "hash" {
		t.Fatalf("configuration lost data: %+v", cfg)
	}
	var raw map[string]json.RawMessage
	if err := readJSON(config, &raw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw["Future"]), "9007199254740993") ||
		!strings.Contains(string(raw["Cores"]), `"FutureKey": true`) {
		t.Fatal("editing dropped unknown fields")
	}
}
