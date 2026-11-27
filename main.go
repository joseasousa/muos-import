package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type Config struct {
	Card, Side, ROMs string
	// Core-specific roots, measured from effective RetroArch settings.
	Cores map[string]Roots
	// Set only after validating both installed builds on hardware.
	StateProfile map[string]string
	// Explicitly approved existing state payload hashes.
	ApprovedStates map[string]bool
	BuildFiles     map[string]string
}
type Roots struct{ Saves, States string }
type Version struct {
	Hash string
	Time int64
	Side string
}
type Entry struct {
	Current    Version
	Seen       map[string]string
	Backups    []Version
	Conflict   map[string]Version
	Generation uint64
	Applied    map[string]uint64
}
type Manifest struct {
	Schema  int
	Entries map[string]*Entry
}
type Game struct{ ID, Name, Core string }

func digest(r io.Reader) (string, error) {
	h := sha256.New()
	_, err := io.Copy(h, r)
	return fmt.Sprintf("%x", h.Sum(nil)), err
}
func coreFor(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".gb", ".gbc":
		return "Gambatte"
	case ".gba":
		return "mGBA"
	}
	return ""
}
func identify(path string) (Game, error) {
	name := filepath.Base(path)
	core := coreFor(name)
	system := strings.ToLower(filepath.Ext(name))
	var input io.ReadCloser
	var err error
	if system == ".zip" {
		z, e := zip.OpenReader(path)
		if e != nil {
			return Game{}, e
		}
		defer z.Close()
		var rom *zip.File
		for _, f := range z.File {
			if !f.Mode().IsRegular() || coreFor(f.Name) == "" {
				continue
			}
			if rom != nil {
				return Game{}, errors.New("ZIP com mais de uma ROM")
			}
			rom = f
		}
		if rom == nil {
			return Game{}, nil
		}
		core = coreFor(rom.Name)
		system = strings.ToLower(filepath.Ext(rom.Name))
		input, err = rom.Open()
	} else if core != "" {
		input, err = os.Open(path)
	} else {
		return Game{}, nil
	}
	if err != nil {
		return Game{}, err
	}
	defer input.Close()
	hash, err := digest(input)
	return Game{system + ":" + hash, strings.TrimSuffix(name, filepath.Ext(name)), core}, err
}

func atomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".savesync-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
func saveJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomic(path, data)
}
func hashBytes(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func object(root string, v Version) ([]byte, error) {
	if len(v.Hash) != 64 || strings.Trim(v.Hash, "0123456789abcdef") != "" {
		return nil, errors.New("hash inválido")
	}
	data, err := os.ReadFile(filepath.Join(root, "objects", v.Hash))
	if err == nil && hashBytes(data) != v.Hash {
		err = errors.New("objeto corrompido")
	}
	return data, err
}
func capture(root, path, side string) (Version, error) {
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return Version{}, errors.New("save não é arquivo regular")
	}
	if err != nil && !os.IsNotExist(err) {
		return Version{}, err
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return Version{}, nil
	}
	if err != nil {
		return Version{}, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return Version{}, err
	}
	if !info.Mode().IsRegular() {
		return Version{}, errors.New("save não é arquivo regular")
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return Version{}, err
	}
	after, err := f.Stat()
	if err != nil {
		return Version{}, err
	}
	if after.Size() != info.Size() || after.ModTime() != info.ModTime() {
		return Version{}, errors.New("save mudou durante a leitura")
	}
	v := Version{hashBytes(data), info.ModTime().Unix(), side}
	if _, err = object(root, v); os.IsNotExist(err) {
		err = atomic(filepath.Join(root, "objects", v.Hash), data)
	}
	return v, err
}

func (e *Entry) backup(v Version) {
	if v.Hash == "" {
		return
	}
	for _, old := range e.Backups {
		if old.Hash == v.Hash {
			return
		}
	}
	e.Backups = append(e.Backups, v)
}
func (e *Entry) publish(v Version) {
	e.backup(e.Current)
	e.Current = v
	e.Generation++
}
func (e *Entry) observe(side string, local Version) {
	previous := e.Seen[side]
	e.Seen[side] = local.Hash
	if local.Hash == "" || local.Hash == e.Current.Hash || local.Hash == previous {
		return
	}
	if len(e.Conflict) > 0 {
		e.backup(e.Conflict[side])
		e.Conflict[side] = local
		return
	}
	if e.Current.Hash == "" || e.Applied[side] == e.Generation {
		e.publish(local)
		return
	}
	if local.Time <= 0 || e.Current.Time <= 0 || local.Time == e.Current.Time {
		e.Conflict = map[string]Version{e.Current.Side: e.Current, side: local}
	} else if local.Time > e.Current.Time {
		e.publish(local)
	} else {
		e.backup(local)
	}
}

func paths(c Config) (map[string]string, error) {
	result := map[string]string{}
	games := map[string][]Game{}
	used := map[string]int{}
	err := filepath.WalkDir(c.ROMs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		g, err := identify(path)
		if err != nil {
			fmt.Printf("Pulado %s (%v)\n", path, err)
			return nil
		}
		if g.ID == "" {
			return nil
		}
		games[g.ID] = append(games[g.ID], g)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for id, list := range games {
		for _, g := range list {
			roots, ok := c.Cores[g.Core]
			if !ok {
				continue
			}
			for _, kind := range []struct {
				root  string
				state bool
			}{{roots.Saves, false}, {roots.States, true}} {
				if kind.root == "" {
					continue
				}
				exts := []string{".srm", ".sav", ".rtc"}
				if kind.state {
					profile := c.StateProfile[g.Core]
					if profile == "" {
						continue
					}
					exts = []string{".state", ".state.auto"}
					for slot := 1; slot <= 999; slot++ {
						exts = append(exts, fmt.Sprintf(".state%d", slot))
					}
				}
				for _, ext := range exts {
					p := filepath.Join(kind.root, g.Name+ext)
					used[strings.ToLower(p)]++
					if len(list) != 1 {
						continue
					}
					key := id + ":" + g.Core + ext
					if kind.state {
						key += ":" + c.StateProfile[g.Core]
					}
					result[key] = p
				}
			}
		}
	}
	for key, p := range result {
		if used[strings.ToLower(p)] > 1 {
			fmt.Println("Pulado por colisão", p)
			delete(result, key)
		}
	}
	for id, list := range games {
		if len(list) > 1 {
			fmt.Println("Pulado por ROM duplicada", id)
		}
	}
	return result, nil
}

// Replaying the pending operation is safe only if the destination still holds
// the expected bytes or the already-installed payload.
type Pending struct {
	Path, Before, Key string
	After             Version
}

func replay(root string, p Pending, allowed map[string]string) error {
	if allowed[p.Key] != p.Path {
		return errors.New("destino pendente não corresponde à configuração")
	}
	local, err := capture(root, p.Path, p.After.Side)
	if err != nil {
		return err
	}
	if local.Hash == p.After.Hash {
		return nil
	}
	if local.Hash != p.Before {
		return errors.New("destino mudou durante operação pendente")
	}
	data, err := object(root, p.After)
	if err != nil {
		return err
	}
	return atomic(p.Path, data)
}

func run(c Config, action, key, hash string) error {
	if action == "check" {
		if err := validateConfig(c, true); err != nil {
			return err
		}
		fmt.Println("Configuração válida")
		return nil
	}
	if runtime.GOOS == "linux" && action != "list" {
		processes, err := filepath.Glob("/proc/[0-9]*/comm")
		if err != nil {
			return err
		}
		for _, path := range processes {
			name, err := os.ReadFile(path)
			if err == nil && strings.HasPrefix(strings.TrimSpace(string(name)), "retroarch") {
				return errors.New("feche o RetroArch antes de sincronizar")
			}
		}
	}
	if err := validateConfig(c, false); err != nil {
		return err
	}
	root := filepath.Join(c.Card, "SaveSync")
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(root, "lock"), os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return err
	}
	if len(c.StateProfile) > 0 {
		if len(c.BuildFiles) < 2 {
			fmt.Println("States desativados: configure hashes do RetroArch e dos cores")
			c.StateProfile = nil
		}
		for path, expected := range c.BuildFiles {
			data, err := os.ReadFile(path)
			if err != nil || hashBytes(data) != expected {
				fmt.Println("States desativados: build mudou", path)
				c.StateProfile = nil
				break
			}
		}
	}
	allowed, err := paths(c)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(root, "manifest.json")
	pendingPath := filepath.Join(root, "pending-"+c.Side+".json")
	m := Manifest{1, map[string]*Entry{}}
	if err = readJSON(manifestPath, &m); err != nil && !os.IsNotExist(err) {
		return err
	}
	if m.Schema != 1 || m.Entries == nil {
		return errors.New("manifesto incompatível")
	}
	var pending Pending
	if err = readJSON(pendingPath, &pending); err == nil {
		if err = replay(root, pending, allowed); err != nil {
			return err
		}
		if err = os.Remove(pendingPath); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if action == "list" {
		for k, e := range m.Entries {
			if allowed[k] == "" {
				continue
			}
			for _, v := range e.Backups {
				fmt.Printf("%s  %s  %s  %s\n", k, v.Hash, v.Side, time.Unix(v.Time, 0).Format(time.RFC3339))
			}
			for _, v := range e.Conflict {
				fmt.Printf("Conflito %s  %s  %s\n", k, v.Hash, v.Side)
			}
		}
		return nil
	}
	if action != "sync" && action != "restore" {
		return errors.New("ação inválida")
	}
	if action == "restore" && allowed[key] == "" {
		return errors.New("save não encontrado")
	}
	for k, path := range allowed {
		if action == "restore" && k != key {
			continue
		}
		e := m.Entries[k]
		if e == nil {
			if action != "restore" && !(strings.Contains(k, ":Gambatte.state") || strings.Contains(k, ":mGBA.state")) {
				if _, err := os.Stat(path); os.IsNotExist(err) {
					continue
				}
			}
			e = &Entry{Seen: map[string]string{}, Applied: map[string]uint64{}}
			m.Entries[k] = e
		}
		local, err := capture(root, path, c.Side)
		if err != nil {
			return err
		}
		previous, observed := e.Seen[c.Side]
		if action == "restore" {
			var selected Version
			for _, v := range e.Backups {
				if v.Hash == hash {
					selected = v
				}
			}
			for _, v := range e.Conflict {
				if v.Hash == hash {
					selected = v
				}
			}
			if selected.Hash == "" {
				return errors.New("backup não encontrado")
			}
			if _, err := object(root, selected); err != nil {
				return err
			}
			e.backup(local)
			for _, v := range e.Conflict {
				e.backup(v)
			}
			e.Conflict = nil
			selected.Side = c.Side
			selected.Time = time.Now().Unix()
			e.publish(selected)
		} else {
			if strings.Contains(k, ":Gambatte.state") || strings.Contains(k, ":mGBA.state") {
				if !observed && local.Hash != "" && local.Hash != e.Current.Hash && !c.ApprovedStates[local.Hash] {
					e.Seen[c.Side] = local.Hash
					fmt.Println("State pulado por origem não confirmada", path)
					continue
				}
			}
			if observed && local.Hash != "" && local.Hash == previous && e.Current.Hash == "" {
				continue
			}
			e.observe(c.Side, local)
		}
		if len(e.Conflict) > 0 {
			fmt.Println("Conflito", path)
		} else if e.Current.Hash != "" && !(local.Hash == "" && previous != "" && e.Applied[c.Side] == e.Generation && action == "sync") {
			if e.Current.Hash != local.Hash {
				e.backup(local)
				// Persist versions and backups before making the journal visible.
				if err = saveJSON(manifestPath, m); err != nil {
					return err
				}
				p := Pending{path, local.Hash, k, e.Current}
				if err = saveJSON(pendingPath, p); err != nil {
					return err
				}
				if err = replay(root, p, allowed); err != nil {
					return err
				}
				e.Seen[c.Side] = e.Current.Hash
				e.Applied[c.Side] = e.Generation
				if err = saveJSON(manifestPath, m); err != nil {
					return err
				}
				if err = os.Remove(pendingPath); err != nil {
					return err
				}
				fmt.Println("Recebido", path)
			} else {
				e.Applied[c.Side] = e.Generation
				if previous != local.Hash {
					fmt.Println("Publicado", path)
				}
			}
		}
		if local.Hash == "" && previous != "" && e.Applied[c.Side] == e.Generation && action == "sync" && e.Seen[c.Side] == "" {
			e.Seen[c.Side] = previous
		}
	}
	if err = saveJSON(manifestPath, m); err != nil {
		return err
	}
	fmt.Println("Sincronização concluída")
	return nil
}
func main() {
	config := flag.String("config", "config.json", "configuração local")
	action := flag.String("action", "sync", "sync, check, list ou restore")
	key := flag.String("key", "", "identificador do save para restaurar")
	hash := flag.String("hash", "", "hash do backup para restaurar")
	yes := flag.Bool("confirm", false, "confirmar restauração")
	ui := flag.Bool("ui", false, "abrir interface no portátil")
	side := flag.String("side", "", "firmware da interface (muos ou knulli)")
	flag.Parse()
	if *ui {
		if err := launchUI(*config, *side); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	var c Config
	err := readJSON(*config, &c)
	if err == nil && *side != "" && c.Side != *side {
		err = fmt.Errorf("Side: esperado %q, encontrado %q", *side, c.Side)
	}
	if err == nil && *side != "" && *action == "sync" {
		err = validateConfig(c, true)
	}
	if err == nil && *action == "restore" && !*yes {
		err = errors.New("restauração requer -confirm")
	}
	if err == nil {
		err = run(c, *action, *key, *hash)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
