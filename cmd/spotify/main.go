package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"unicode"
)

func help() {
	fmt.Println("Manage Spotify playlists, Liked Songs, history and playback.\nUsage: spotify [--json] COMMAND\nCommands:")
	for _, k := range slices.Sorted(maps.Keys(specs)) {
		fmt.Println(" ", specs[k].usage)
	}
	fmt.Println("Use favorites for Liked Songs. Playlist names match exactly, ignoring case. Use an ID for duplicate names. Recent history includes up to 50 plays.")
}
func splitLine(s string) []string {
	var words []string
	var word strings.Builder
	var quote rune
	started, escaped := false, false
	for _, r := range s {
		switch {
		case escaped:
			word.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
			started = true
		case quote != 0:
			if quote == r {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			started = true
		case unicode.IsSpace(r):
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
		default:
			started = true
			word.WriteRune(r)
		}
	}
	if quote != 0 || escaped {
		fail("unfinished quote or escape")
	}
	if started {
		words = append(words, word.String())
	}
	return words
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
	if len(a) == 0 {
		help()
		stat, e := os.Stdin.Stat()
		check(e)
		if stat.Mode()&os.ModeCharDevice == 0 {
			return
		}
		fmt.Fprint(os.Stderr, "Spotify command (or q to quit)> ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		a = splitLine(line)
		if len(a) == 0 || a[0] == "q" {
			return
		}
		if a[0] == "--json" {
			machine = true
			a = a[1:]
		}
		if len(a) == 0 {
			return
		}
	}
	if a[0] == "--help" || a[0] == "-h" {
		help()
		return
	}
	if len(a) == 2 && (a[1] == "--help" || a[1] == "-h") {
		s, ok := specs[a[0]]
		if !ok {
			fail("unknown command %q", a[0])
		}
		fmt.Println("Usage: spotify", s.usage)
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
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, e := os.UserHomeDir()
		check(e)
		base = filepath.Join(home, ".config")
	}
	folder := filepath.Join(base, "spotify-mover")
	check(os.MkdirAll(folder, 0700))
	lock, e := os.OpenFile(filepath.Join(folder, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	check(e)
	defer lock.Close()
	check(syscall.Flock(int(lock.Fd()), syscall.LOCK_EX))
	c := newClient(folder, clientID(folder, cmd.name == "login"))
	output(c.run(cmd), machine)
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
