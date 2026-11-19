package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
)

func help() {
	fmt.Println("Manage Spotify playlists, Liked Songs, history and playback.\nUsage: spotifyctl [--json] COMMAND\nCommands:")
	for _, k := range slices.Sorted(maps.Keys(specs)) {
		fmt.Println(" ", specs[k].usage)
	}
	fmt.Println("Use favorites for Liked Songs. Playlist names match exactly, ignoring case. Use an ID for duplicate names. Recent history includes up to 50 plays.")
}
func loadEnv(path string) {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return
	}
	check(e)
	comment := regexp.MustCompile(`\s+#.*$`)
	for _, line := range strings.Split(strings.TrimPrefix(string(b), "\ufeff"), "\n") {
		key, v, ok := strings.Cut(strings.TrimPrefix(strings.TrimSpace(line), "export "), "=")
		key = strings.TrimSpace(key)
		if !ok || (key != "SPOTIPY_CLIENT_ID" && key != "SPOTIFY_NORM") {
			continue
		}
		if _, set := os.LookupEnv(key); set {
			continue
		}
		v = strings.TrimSpace(comment.ReplaceAllString(strings.TrimSpace(v), ""))
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[0] == v[len(v)-1] {
			v = v[1 : len(v)-1]
		}
		check(os.Setenv(key, v))
	}
}
func output(result any, machine bool) {
	if machine {
		check(json.NewEncoder(os.Stdout).Encode(result))
		return
	}
	printRow := func(r obj, labels bool) {
		parts := []string{}
		// Keep the display order of the source CLI for human readers and shell callers.
		order := strings.Fields("played_at id name artists album active status title device progress_ms volume source destination track added forward reverse command accepted")
		for _, key := range order {
			if r[key] == nil {
				continue
			}
			v := fmt.Sprint(r[key])
			if labels {
				v = key + "=" + v
			}
			parts = append(parts, v)
		}
		fmt.Println(strings.Join(parts, "  "))
	}
	switch r := result.(type) {
	case obj:
		printRow(r, true)
	case []obj:
		if len(r) == 0 {
			fmt.Println("No results.")
		}
		for _, v := range r {
			printRow(v, false)
		}
	}
}
func execute() {
	a := os.Args[1:]
	machine := len(a) > 0 && a[0] == "--json"
	if machine {
		a = a[1:]
	}
	if len(a) == 0 || a[0] == "--help" || a[0] == "-h" {
		help()
		return
	}
	if len(a) == 2 && (a[1] == "--help" || a[1] == "-h") {
		s, ok := specs[a[0]]
		if !ok {
			fail("unknown command %q", a[0])
		}
		fmt.Println("Usage: spotifyctl", s.usage)
		return
	}
	cmd := parse(a)
	exe, e := os.Executable()
	check(e)
	if env := os.Getenv("SPOTIFY_CHECKOUT_ENV"); env != "" {
		loadEnv(env)
	} else {
		loadEnv(filepath.Join(filepath.Dir(exe), ".env"))
	}
	folder := configDir()
	check(os.MkdirAll(folder, 0700))
	lock, e := os.OpenFile(filepath.Join(folder, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		fail("open Spotify configuration lock in %s: %v (set SPOTIFYCTL_CONFIG_DIR to a writable private directory)", folder, e)
	}
	defer lock.Close()
	check(syscall.Flock(int(lock.Fd()), syscall.LOCK_EX))
	c := newClient(folder, clientID(folder, cmd.name == "login"))
	output(c.run(cmd), machine)
}
func configDir() string {
	if folder := os.Getenv("SPOTIFYCTL_CONFIG_DIR"); folder != "" {
		return folder
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, e := os.UserHomeDir()
		check(e)
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "spotifyctl")
}

func main() {
	defer func() {
		if problem := recover(); problem != nil {
			if e, ok := problem.(error); ok {
				fmt.Fprintln(os.Stderr, e)
				os.Exit(1)
			}
			panic(problem)
		}
	}()
	execute()
}
