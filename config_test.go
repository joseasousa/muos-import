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
				{"Side", func(c *Config) { c.Side = "outro" }},
				{"Card", func(c *Config) { c.Card = "relativo" }},
				{"ROMs", func(c *Config) { c.ROMs = marker }},
				{"ROMs", func(c *Config) { c.ROMs = filepath.Join(roms, "ausente") }},
				{"Cores.mGBA.Saves", func(c *Config) { c.Cores = map[string]Roots{"mGBA": {Saves: "relativo"}} }},
				{"Cores.mGBA.States", func(c *Config) { c.Cores = map[string]Roots{"mGBA": {Saves: saves, States: marker}} }},
				{"Cores", func(c *Config) { c.Cores = map[string]Roots{"outro": {Saves: saves}} }},
			} {
				bad := cfg
				tc.edit(&bad)
				if err := run(bad, "check", "", ""); err == nil || !strings.Contains(err.Error(), tc.field) {
					t.Errorf("check de %s retornou %v", tc.field, err)
				}
			}
			if _, err := os.Stat(filepath.Join(card, "SaveSync")); !os.IsNotExist(err) {
				t.Fatalf("check alterou o cartão: %v", err)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
			if err := run(cfg, "check", "", ""); err == nil || !strings.Contains(err.Error(), ".savesync-card") {
				t.Fatalf("check sem marcador retornou %v", err)
			}
		})
	}
}

func TestHandheldConfiguration(t *testing.T) {
	config := filepath.Join(t.TempDir(), "config.json")
	h := newHandheld(config, "muos")
	if h.screen != uiConfig || h.editor.config.Side != "muos" {
		t.Fatal("primeira abertura não iniciou a configuração do muOS")
	}
	h.activate()
	if h.screen != uiBrowse {
		t.Fatal("seletor de diretórios não abriu")
	}
	h.browse(filepath.Dir(config))
	h.activate()
	if h.editor.config.Card != filepath.Dir(config) || h.screen != uiConfig {
		t.Fatal("pasta selecionada não foi aplicada ao formulário")
	}
	h.cursor = len(h.fields())
	h.activate()
	if h.screen != uiSave {
		t.Fatal("faltou confirmação antes de salvar")
	}
	h.back()
	h.back()
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatalf("edição cancelada gravou configuração: %v", err)
	}
	h.cursor = 2
	h.activate()
	if h.screen != uiResult || h.success || h.job != nil {
		t.Fatal("sincronização sem configuração não exibiu a falha")
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
		t.Fatalf("configuração perdeu dados: %+v", cfg)
	}
	var raw map[string]json.RawMessage
	if err := readJSON(config, &raw); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw["Future"]), "9007199254740993") ||
		!strings.Contains(string(raw["Cores"]), `"FutureKey": true`) {
		t.Fatal("edição perdeu campos desconhecidos")
	}
}
