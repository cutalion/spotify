package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const scopes = "user-library-read user-library-modify playlist-read-private playlist-modify-private playlist-modify-public user-read-recently-played user-read-playback-state user-modify-playback-state"
const redirect = "http://127.0.0.1:8888/callback"

var defaultClientID string // Optional public client ID set by the release builder.
type Token struct {
	Access   string `json:"access_token"`
	Refresh  string `json:"refresh_token"`
	Expires  int64  `json:"expires_at"`
	Lifetime int64  `json:"expires_in"`
	Scope    string `json:"scope"`
	Client   string `json:"client_id,omitempty"`
}

func readJSON(path string, v any) bool {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return false
	}
	check(e)
	check(json.Unmarshal(b, v))
	return true
}
func writeJSON(path string, v any) {
	b, e := json.Marshal(v)
	check(e)
	f, e := os.CreateTemp(filepath.Dir(path), ".spotify-*")
	check(e)
	defer os.Remove(f.Name())
	_, e = f.Write(b)
	closeErr := f.Close()
	check(e)
	check(closeErr)
	check(os.Rename(f.Name(), path))
}

type config struct {
	ID string `json:"client_id"`
}

func clientID(folder string, login bool) string {
	if v := os.Getenv("SPOTIPY_CLIENT_ID"); v != "" {
		return v
	}
	var cfg config
	path := filepath.Join(folder, "config.json")
	readJSON(path, &cfg)
	if cfg.ID != "" {
		return cfg.ID
	}
	if defaultClientID != "" {
		return defaultClientID
	}
	if !login {
		fail("run spotifyctl login to configure your Spotify client ID")
	}
	fmt.Fprintln(os.Stderr, "Create a Web API app at https://developer.spotify.com/dashboard and register "+redirect)
	fmt.Fprint(os.Stderr, "Client ID: ")
	scan := bufio.NewScanner(os.Stdin)
	if !scan.Scan() {
		fail("a client ID is required")
	}
	cfg.ID = strings.TrimSpace(scan.Text())
	if cfg.ID == "" {
		fail("a client ID is required")
	}
	writeJSON(path, cfg)
	return cfg.ID
}
func freshSecret() string {
	var b [32]byte
	_, e := rand.Read(b[:])
	check(e)
	return base64.RawURLEncoding.EncodeToString(b[:])
}
func (c *client) authorize() url.Values {
	verifier, state := freshSecret(), freshSecret()
	hash := sha256.Sum256([]byte(verifier))
	q := url.Values{"client_id": {c.id}, "response_type": {"code"}, "redirect_uri": {redirect}, "scope": {scopes}, "state": {state}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(hash[:])}}
	listener, e := net.Listen("tcp", "127.0.0.1:8888")
	check(e)
	type answer struct{ code, problem string }
	result := make(chan answer, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		v := r.URL.Query()
		if r.Method != "GET" || subtle.ConstantTimeCompare([]byte(state), []byte(v.Get("state"))) != 1 {
			http.Error(w, "Invalid callback", 400)
			return
		}
		a := answer{v.Get("code"), v.Get("error")}
		if a.code == "" && a.problem == "" {
			http.Error(w, "Missing code", 400)
			return
		}
		fmt.Fprintln(w, "Return to the Spotify CLI.")
		select {
		case result <- a:
		default:
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go server.Serve(listener)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if server.Shutdown(ctx) != nil {
			server.Close()
		}
	}()
	link := "https://accounts.spotify.com/authorize?" + q.Encode()
	fmt.Fprintln(os.Stderr, "Authorize Spotify in your browser:", link)
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	cmd := exec.Command(opener, link)
	if cmd.Start() == nil {
		go cmd.Wait()
	}
	select {
	case a := <-result:
		if a.problem != "" {
			fail("Spotify authorization: %s", a.problem)
		}
		return url.Values{"grant_type": {"authorization_code"}, "code": {a.code}, "redirect_uri": {redirect}, "code_verifier": {verifier}}
	case <-time.After(5 * time.Minute):
		fail("Spotify login timed out")
	}
	return nil
}
func authorized(scope string) bool {
	fields := strings.Fields(scope)
	for _, s := range strings.Fields(scopes) {
		if !slices.Contains(fields, s) {
			return false
		}
	}
	return true
}
func (c *client) access() string {
	path := filepath.Join(c.folder, "token.json")
	if c.token.Access == "" {
		readJSON(path, &c.token)
	}
	old := c.token
	if old.Client != "" && old.Client != c.id {
		old = Token{}
	}
	var form url.Values
	if authorized(old.Scope) {
		if old.Access != "" && old.Expires > time.Now().Unix()+60 {
			return old.Access
		}
		if old.Refresh != "" {
			form = url.Values{"grant_type": {"refresh_token"}, "refresh_token": {old.Refresh}}
		}
	}
	for {
		if form == nil {
			old = Token{}
			form = c.authorize()
		}
		form.Set("client_id", c.id)
		res, e := c.http.PostForm("https://accounts.spotify.com/api/token", form)
		check(e)
		var t struct {
			Token
			Error string `json:"error"`
		}
		e = json.NewDecoder(res.Body).Decode(&t)
		res.Body.Close()
		check(e)
		if t.Error == "invalid_grant" && form.Get("grant_type") == "refresh_token" {
			form = nil
			continue
		}
		if res.StatusCode != 200 || t.Access == "" {
			fail("Spotify token request: %s %s", res.Status, t.Error)
		}
		if t.Refresh == "" {
			t.Refresh = old.Refresh
		}
		if t.Scope == "" {
			t.Scope = old.Scope
		}
		if !authorized(t.Scope) {
			fail("Spotify did not grant the required scopes")
		}
		t.Client = c.id
		t.Expires = time.Now().Unix() + t.Lifetime
		c.token = t.Token
		writeJSON(path, c.token)
		return t.Access
	}
}
