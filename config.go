package main

import (
	"fmt"
	"os"
	"path/filepath"
)

var supportedCores = [...]string{"Gambatte", "mGBA"}

func validSide(side string) bool {
	return side == "muos" || side == "knulli"
}

func validateConfig(c Config, checkDirectories bool) error {
	if !validSide(c.Side) {
		return fmt.Errorf("Side: invalid firmware (%q)", c.Side)
	}
	checkPath := func(field, path string) error {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("%s: provide an absolute path (%q)", field, path)
		}
		if !checkDirectories {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s: %q is not a directory", field, path)
		}
		return nil
	}
	if err := checkPath("Card", c.Card); err != nil {
		return err
	}
	if err := checkPath("ROMs", c.ROMs); err != nil {
		return err
	}
	marker := filepath.Join(c.Card, ".savesync-card")
	info, err := os.Stat(marker)
	if err != nil {
		return fmt.Errorf("Card: SD2 could not be verified: %w", err)
	}
	if checkDirectories && !info.Mode().IsRegular() {
		return fmt.Errorf("Card: %q must be a file", marker)
	}
	for core, roots := range c.Cores {
		if roots.Saves != "" {
			if err := checkPath("Cores."+core+".Saves", roots.Saves); err != nil {
				return err
			}
		}
		if roots.States != "" {
			if err := checkPath("Cores."+core+".States", roots.States); err != nil {
				return err
			}
		}
	}
	if checkDirectories {
		for _, core := range supportedCores {
			if c.Cores[core].Saves != "" {
				return nil
			}
		}
		return fmt.Errorf("Cores: choose a save folder for Gambatte or mGBA")
	}
	return nil
}
