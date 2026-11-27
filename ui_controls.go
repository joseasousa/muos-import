//go:build sdl && cgo

package main

import (
	"encoding/xml"
	"os"
	"strconv"
	"strings"
)

// KNULLI's EmulationStation names the face buttons by their printed labels.
func stationMapping(path, guid, name string) string {
	var doc struct {
		Controllers []struct {
			Name   string `xml:"deviceName,attr"`
			GUID   string `xml:"deviceGUID,attr"`
			Type   string `xml:"type,attr"`
			Inputs []struct {
				Name  string `xml:"name,attr"`
				Type  string `xml:"type,attr"`
				ID    int    `xml:"id,attr"`
				Value int    `xml:"value,attr"`
			} `xml:"input"`
		} `xml:"inputConfig"`
	}
	data, err := os.ReadFile(path)
	if err != nil || xml.Unmarshal(data, &doc) != nil {
		return ""
	}
	for _, controller := range doc.Controllers {
		if controller.Type != "joystick" || controller.GUID != guid || controller.Name != name {
			continue
		}
		bindings := map[string]string{}
		for _, input := range controller.Inputs {
			if input.ID < 0 {
				continue
			}
			id := strconv.Itoa(input.ID)
			switch input.Type {
			case "button":
				bindings[input.Name] = "b" + id
			case "hat":
				bindings[input.Name] = "h" + id + "." + strconv.Itoa(input.Value)
			case "axis":
				if input.Value == -1 || input.Value == 1 {
					sign := "+"
					if input.Value < 0 {
						sign = "-"
					}
					bindings[input.Name] = sign + "a" + id
				}
			}
		}
		fields := []string{guid, strings.ReplaceAll(name, ",", " ")}
		for _, key := range [][2]string{{"a", "a"}, {"b", "b"}, {"up", "dpup"}, {"down", "dpdown"}} {
			binding := bindings[key[0]]
			if binding == "" {
				return ""
			}
			fields = append(fields, key[1]+":"+binding)
		}
		return strings.Join(fields, ",") + ",platform:Linux,"
	}
	return ""
}
