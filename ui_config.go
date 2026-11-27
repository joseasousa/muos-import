package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type configEditor struct {
	config Config
	raw    map[string]json.RawMessage
}

func openConfigEditor(path, side string) (configEditor, error) {
	editor := configEditor{config: Config{Side: side}, raw: map[string]json.RawMessage{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return editor, nil
	}
	if err != nil {
		return editor, err
	}
	if err = json.Unmarshal(data, &editor.raw); err != nil {
		return editor, err
	}
	if editor.raw == nil {
		return editor, fmt.Errorf("configuration must be a JSON object")
	}
	editor.config = Config{}
	if err = json.Unmarshal(data, &editor.config); err != nil {
		return editor, err
	}
	return editor, nil
}

// Replace case-insensitive aliases of edited keys, as encoding/json accepts them.
func setConfigField(raw map[string]json.RawMessage, key string, value any) {
	for name := range raw {
		if strings.EqualFold(name, key) {
			delete(raw, name)
		}
	}
	// All callers provide strings or maps of already validated JSON.
	raw[key], _ = json.Marshal(value)
}

func (e configEditor) save(path string) error {
	setConfigField(e.raw, "Side", e.config.Side)
	setConfigField(e.raw, "Card", e.config.Card)
	setConfigField(e.raw, "ROMs", e.config.ROMs)
	cores := map[string]json.RawMessage{}
	for name, value := range e.raw {
		if strings.EqualFold(name, "Cores") {
			var entries map[string]json.RawMessage
			if err := json.Unmarshal(value, &entries); err != nil {
				return err
			}
			for core, entry := range entries {
				cores[core] = entry
			}
		}
	}
	for _, core := range supportedCores {
		fields := map[string]json.RawMessage{}
		if value, ok := cores[core]; ok {
			if err := json.Unmarshal(value, &fields); err != nil {
				return err
			}
			if fields == nil {
				fields = map[string]json.RawMessage{}
			}
		}
		setConfigField(fields, "Saves", e.config.Cores[core].Saves)
		cores[core], _ = json.Marshal(fields)
	}
	setConfigField(e.raw, "Cores", cores)
	return saveJSON(path, e.raw)
}
