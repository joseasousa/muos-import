package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func put(t *testing.T, path, value string, when int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(when, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}
func check(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != want {
		t.Fatalf("%s = %q, %v, want %q", path, data, err, want)
	}
}
func TestRoundTripAndRestore(t *testing.T) {
	root := t.TempDir()
	card := filepath.Join(root, "card")
	put(t, filepath.Join(card, ".savesync-card"), "", 1)
	configs := []Config{}
	names := []string{"Game (muOS)", "Game (KNULLI)"}
	for i, side := range []string{"muos", "knulli"} {
		base := filepath.Join(root, side)
		c := Config{Side: side, Card: card, ROMs: filepath.Join(base, "roms"), Cores: map[string]Roots{"mGBA": {Saves: filepath.Join(base, "saves")}}}
		put(t, filepath.Join(c.ROMs, names[i]+".gba"), "same rom", 1)
		configs = append(configs, c)
	}
	a := filepath.Join(configs[0].Cores["mGBA"].Saves, names[0]+".srm")
	b := filepath.Join(configs[1].Cores["mGBA"].Saves, names[1]+".srm")
	sync := func(i int) {
		t.Helper()
		if err := run(configs[i], "sync", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	put(t, a, "first", 100)
	sync(0)
	sync(1)
	check(t, b, "first")
	put(t, a, "older", 200)
	put(t, b, "newer", 300)
	sync(0)
	sync(1)
	sync(0)
	check(t, a, "newer")
	key := ".gba:" + hashBytes([]byte("same rom")) + ":mGBA.srm"
	if err := run(configs[0], "restore", key, hashBytes([]byte("older"))); err != nil {
		t.Fatal(err)
	}
	sync(1)
	check(t, b, "older")
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	sync(1)
	sync(1)
	if _, err := os.Stat(b); !os.IsNotExist(err) {
		t.Fatalf("deleted save returned: %v", err)
	}
}
func TestPendingReplay(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "active")
	source := filepath.Join(root, "source")
	put(t, path, "old", 100)
	put(t, source, "new", 200)
	old, err := capture(root, path, "muos")
	if err != nil {
		t.Fatal(err)
	}
	next, err := capture(root, source, "knulli")
	if err != nil {
		t.Fatal(err)
	}
	p := Pending{path, old.Hash, "key", next}
	for i := 0; i < 2; i++ {
		if err := replay(root, p, map[string]string{"key": path}); err != nil {
			t.Fatal(err)
		}
	}
	check(t, path, "new")
	check(t, filepath.Join(root, "objects", old.Hash), "old")
	put(t, path, "changed", 300)
	if err := replay(root, p, map[string]string{"key": path}); err == nil {
		t.Fatal("overwrote external change")
	}
	check(t, path, "changed")
}

func TestEqualDatesPreserveBoth(t *testing.T) {
	e := Entry{Seen: map[string]string{}, Applied: map[string]uint64{}}
	a := Version{"aaa", 100, "muos"}
	b := Version{"bbb", 100, "knulli"}
	e.observe("muos", a)
	e.Applied["muos"] = e.Generation
	e.observe("knulli", b)
	if e.Current != a || e.Conflict["muos"] != a || e.Conflict["knulli"] != b {
		t.Fatalf("conflict lost versions: %+v", e)
	}
}

func TestMissingCardLeavesStoresUntouched(t *testing.T) {
	root := t.TempDir()
	c := Config{Side: "muos", Card: filepath.Join(root, "missing"), ROMs: root}
	if err := run(c, "sync", "", ""); err == nil {
		t.Fatal("accepted an unconfirmed card")
	}
	if _, err := os.Stat(c.Card); !os.IsNotExist(err) {
		t.Fatalf("modified the missing mount: %v", err)
	}
}
