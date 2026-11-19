package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) *http.Response

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r), nil }
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Status: http.StatusText(code), Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestQueueTrack(t *testing.T) {
	c := newClient(t.TempDir(), "example")
	c.token = Token{Access: "test", Scope: scopes, Expires: time.Now().Unix() + 3600}
	const track = "abcdefghijklmnopqrstuv"
	c.http.Transport = transport(func(r *http.Request) *http.Response {
		query := r.URL.Query()
		if r.Method != http.MethodPost || r.URL.Path != "/v1/me/player/queue" ||
			query.Get("uri") != "spotify:track:"+track || query.Get("device_id") != "speaker" {
			t.Fatalf("unexpected queue request: %s %s", r.Method, r.URL)
		}
		return response(http.StatusNoContent, "")
	})
	got := c.run(parse([]string{"queue", "https://open.spotify.com/track/" + track, "--device", "speaker"})).(obj)
	if got["command"] != "queue" || got["accepted"] != true {
		t.Fatal(got)
	}
}

func TestListQueue(t *testing.T) {
	c := newClient(t.TempDir(), "example")
	c.token = Token{Access: "test", Scope: scopes, Expires: time.Now().Unix() + 3600}
	c.http.Transport = transport(func(r *http.Request) *http.Response {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/me/player/queue" {
			t.Fatalf("unexpected queue request: %s %s", r.Method, r.URL)
		}
		return response(http.StatusOK, `{"currently_playing":{"name":"Current"},"queue":[{"name":"Next","uri":"spotify:track:next"},{"name":"Later","uri":"spotify:episode:later"}]}`)
	})
	got := c.run(parse([]string{"queue"})).([]obj)
	if len(got) != 2 || got[0]["name"] != "Next" || got[1]["uri"] != "spotify:episode:later" {
		t.Fatal(got)
	}
}

func TestLegacyRefreshAndStatus(t *testing.T) {
	c := newClient(t.TempDir(), "example")
	path := filepath.Join(c.folder, "token.json")
	writeJSON(path, obj{"access_token": "expired", "refresh_token": "legacy", "scope": scopes, "expires_at": 1})
	c.http.Transport = transport(func(r *http.Request) *http.Response {
		switch r.URL.Path {
		case "/api/token":
			if r.FormValue("refresh_token") != "legacy" || r.FormValue("client_id") != "example" {
				t.Fatal("unexpected refresh form")
			}
			return response(200, `{"access_token":"fresh","expires_in":3600}`)
		case "/v1/me/player":
			if r.Header.Get("Authorization") != "Bearer fresh" {
				t.Fatal("missing refreshed token")
			}
			return response(204, "")
		default:
			t.Fatalf("unexpected request: %v", r.URL.Path)
			return nil
		}
	})
	got := c.run(parse([]string{"status"})).(obj)
	if got["status"] != "idle" {
		t.Fatal(got)
	}
	var token Token
	readJSON(path, &token)
	if token.Refresh != "legacy" || token.Scope != scopes || token.Client != "example" {
		t.Fatal("refresh lost cached fields")
	}
}

func TestBrowserLogin(t *testing.T) {
	folder := t.TempDir()
	c := newClient(folder, "example")
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	recorded := filepath.Join(folder, "url")
	t.Setenv("RECORDED_URL", recorded)
	t.Setenv("PATH", folder)
	if err := os.WriteFile(filepath.Join(folder, name), []byte("#!/bin/sh\nprintf '%s' \"$1\" > \"$RECORDED_URL\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	challenge := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				done <- fmt.Errorf("%v", e)
			}
		}()
		c.http.Transport = transport(func(r *http.Request) *http.Response {
			h := sha256.Sum256([]byte(r.FormValue("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(h[:]) != <-challenge || r.FormValue("code") != "test-code" || r.FormValue("redirect_uri") != redirect {
				panic("incorrect PKCE exchange")
			}
			return response(200, fmt.Sprintf(`{"access_token":"test","expires_in":3600,"scope":%q}`, scopes))
		})
		c.run(parse([]string{"login"}))
		done <- nil
	}()
	var link *url.URL
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		select {
		case err := <-done:
			t.Fatalf("login ended before callback: %v", err)
		default:
		}
		data, _ := os.ReadFile(recorded)
		if len(data) > 0 {
			var e error
			link, e = url.Parse(string(data))
			if e != nil {
				t.Fatal(e)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if link == nil {
		t.Fatal("browser did not launch")
	}
	q := link.Query()
	if q.Get("state") == "" || q.Get("code_challenge_method") != "S256" {
		t.Fatal("invalid authorization URL")
	}
	challenge <- q.Get("code_challenge")
	for _, state := range []string{"wrong", q.Get("state")} {
		res, e := http.Get(redirect + "?code=test-code&state=" + state)
		if e != nil {
			t.Fatal(e)
		}
		res.Body.Close()
		if state == "wrong" && res.StatusCode != 400 {
			t.Fatal("invalid state accepted")
		}
	}
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("login timed out")
	}
}

func TestResumeLegacyTransfer(t *testing.T) {
	c := newClient(t.TempDir(), "example")
	c.token = Token{Access: "test", Scope: scopes, Expires: time.Now().Unix() + 3600}
	const uri = "spotify:track:abcdefghijklmnopqrstuv"
	const dest = "0000000000000000000000"
	path := filepath.Join(c.folder, "transfer-"+dest+"-favorites.json")
	writeJSON(path, obj{"operation": []string{"move-all", "favorites", dest}, "forward": []string{uri}, "reverse": []string{}})
	deleted := false
	c.http.Transport = transport(func(r *http.Request) *http.Response {
		switch {
		case r.Method == "GET":
			return response(200, `{"items":[{"track":{"uri":"`+uri+`"}}],"next":null}`)
		case r.Method == "DELETE" && r.URL.Path == "/v1/me/library" && r.URL.Query().Get("uris") == uri:
			deleted = true
			return response(204, "")
		default:
			t.Fatal("unexpected mutation", r.Method, r.URL.Path)
			return nil
		}
	})
	c.run(parse([]string{"move-all", "--from", "favorites", "--to", dest}))
	if _, err := os.Stat(path); !os.IsNotExist(err) || !deleted {
		t.Fatal("transfer did not finish")
	}
}

func TestClientIDSetup(t *testing.T) {
	folder := t.TempDir()
	t.Setenv("SPOTIPY_CLIENT_ID", "")
	input, err := os.OpenFile(filepath.Join(folder, "input"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if _, err = fmt.Fprintln(input, "my-app"); err != nil {
		t.Fatal(err)
	}
	if _, err = input.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	original := os.Stdin
	os.Stdin = input
	t.Cleanup(func() { os.Stdin = original })
	if got := clientID(folder, true); got != "my-app" {
		t.Fatalf("interactive login got %q", got)
	}
	if got := clientID(folder, false); got != "my-app" {
		t.Fatalf("saved setup got %q", got)
	}
	t.Setenv("SPOTIPY_CLIENT_ID", "override")
	if got := clientID(folder, false); got != "override" {
		t.Fatalf("environment override got %q", got)
	}
}

func TestPauseAcceptsPlainTextAcknowledgement(t *testing.T) {
	c := newClient(t.TempDir(), "example")
	c.token = Token{Access: "test", Scope: scopes, Expires: time.Now().Unix() + 3600}
	c.http.Transport = transport(func(r *http.Request) *http.Response {
		if r.Method != "PUT" || r.URL.Path != "/v1/me/player/pause" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		return response(200, "success")
	})
	result := c.run(parse([]string{"pause"})).(obj)
	if result["accepted"] != true {
		t.Fatal(result)
	}
}

func TestResumeSingleIdleDevice(t *testing.T) {
	c := newClient(t.TempDir(), "example")
	c.token = Token{Access: "test", Scope: scopes, Expires: time.Now().Unix() + 3600}
	transferred := false
	c.http.Transport = transport(func(r *http.Request) *http.Response {
		switch r.URL.Path {
		case "/v1/me/player/play":
			return response(404, `{"error":{"reason":"NO_ACTIVE_DEVICE"}}`)
		case "/v1/me/player/devices":
			return response(200, `{"devices":[{"id":"speaker","is_active":false}]}`)
		case "/v1/me/player":
			body, _ := io.ReadAll(r.Body)
			transferred = r.Method == "PUT" && string(body) == `{"device_ids":["speaker"],"play":true}`
			return response(204, "")
		default:
			t.Fatal(r.URL.Path)
			return nil
		}
	})
	c.run(parse([]string{"play"}))
	if !transferred {
		t.Fatal("expected transfer and resume")
	}
}

func TestRemovePlaylist(t *testing.T) {
	const id = "3cEYpjA9oz9GiPac4AsH4n"
	c := newClient(t.TempDir(), "example")
	c.token = Token{Access: "test", Scope: scopes, Expires: time.Now().Unix() + 3600}
	removed := false
	c.http.Transport = transport(func(r *http.Request) *http.Response {
		if r.Method != "DELETE" || r.URL.Path != "/v1/me/library" || r.URL.Query().Get("uris") != "spotify:playlist:"+id {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL)
		}
		removed = true
		return response(200, "")
	})
	result := c.run(parse([]string{"remove", id})).(obj)
	if !removed || result["id"] != id || result["status"] != "removed" {
		t.Fatal(result)
	}
}
