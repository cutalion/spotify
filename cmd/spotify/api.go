package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

type obj = map[string]any
type client struct {
	folder, id string
	token      Token
	base       string
	http       *http.Client
}

func object(v any) obj { o, _ := v.(map[string]any); return o }
func rows(v any) []obj {
	out := []obj{}
	a, _ := v.([]any)
	for _, e := range a {
		if o := object(e); o != nil {
			out = append(out, o)
		}
	}
	return out
}
func str(v any) string { s, _ := v.(string); return s }
func check(err error) {
	if err != nil {
		panic(err)
	}
}
func fail(f string, a ...any) { panic(fmt.Errorf(f, a...)) }
func (c *client) request(method, path string, q url.Values, body any) obj {
	return c.requestOnce(method, path, q, body, true)
}

func (c *client) requestOnce(method, path string, q url.Values, body any, recoverDevice bool) obj {
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		check(err)
	}
	token := c.access()
	req, err := http.NewRequest(method, c.base+path+"?"+q.Encode(), bytes.NewReader(data))
	check(err)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	check(err)
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	check(err)
	// An idle Connect device can reject resume until playback is transferred to it.
	if res.StatusCode == http.StatusNotFound && method == "PUT" && path == "/me/player/play" && recoverDevice {
		var failure struct{ Error struct{ Reason string } }
		if json.Unmarshal(b, &failure) == nil && failure.Error.Reason == "NO_ACTIVE_DEVICE" {
			if device := c.idlePlaybackTarget(q.Get("device_id")); device != "" {
				c.request("PUT", "/me/player", nil, obj{"device_ids": []string{device}, "play": body == nil})
				if body != nil {
					q.Set("device_id", device)
					return c.requestOnce(method, path, q, body, false)
				}
				return nil
			}
		}
	}
	if res.StatusCode/100 != 2 {
		fail("Spotify %s %s: %s %s", method, path, res.Status, strings.TrimSpace(string(b)))
	}
	// Player controls acknowledge the command and have no response model.
	if method != "GET" && (path == "/me/player" || strings.HasPrefix(path, "/me/player/")) {
		return nil
	}
	var result obj
	if len(b) > 0 {
		check(json.Unmarshal(b, &result))
	}
	return result
}
func (c *client) pages(path string) []obj {
	all := []obj{}
	for offset := 0; ; {
		p := c.request("GET", path, url.Values{"limit": {"50"}, "offset": {strconv.Itoa(offset)}}, nil)
		items, _ := p["items"].([]any)
		all = append(all, rows(items)...)
		if p["next"] == nil || len(items) == 0 {
			return all
		}
		offset += len(items)
	}
}

var identifier = regexp.MustCompile("^[a-zA-Z0-9]{22}$")

func id(s, kind string) string {
	if u, err := url.Parse(s); err == nil && u.Hostname() == "open.spotify.com" {
		p := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(p) == 3 && strings.HasPrefix(p[0], "intl-") {
			p = p[1:]
		}
		if len(p) == 2 && p[0] == kind {
			s = p[1]
		}
	}
	s = strings.TrimPrefix(s, "spotify:"+kind+":")
	if identifier.MatchString(s) {
		return s
	}
	return ""
}
func requiredID(s, k string) string {
	v := id(s, k)
	if v == "" {
		fail("expected a Spotify %s ID, URI or link", k)
	}
	return v
}
func (c *client) resolve(name string) string {
	if strings.EqualFold(name, "favorites") {
		return "favorites"
	}
	if strings.EqualFold(name, "norm") && os.Getenv("SPOTIFY_NORM") != "" {
		return requiredID(os.Getenv("SPOTIFY_NORM"), "playlist")
	}
	if v := id(name, "playlist"); v != "" {
		return v
	}
	matches := []string{}
	for _, p := range c.pages("/me/playlists") {
		if strings.EqualFold(str(p["name"]), name) {
			matches = append(matches, str(p["id"]))
		}
	}
	if len(matches) != 1 {
		fail("%q matched %d playlists. Run playlists and use an ID.", name, len(matches))
	}
	return matches[0]
}
func (c *client) tracks(collection string) []obj {
	path := "/playlists/" + collection + "/items"
	if collection == "favorites" {
		path = "/me/tracks"
	}
	out := []obj{}
	for _, r := range c.pages(path) {
		t := object(r["item"])
		if t == nil {
			t = object(r["track"])
		}
		if strings.HasPrefix(str(t["uri"]), "spotify:track:") && t["is_local"] != true {
			out = append(out, t)
		}
	}
	return out
}
func summary(t obj) obj {
	names := []string{}
	for _, a := range rows(t["artists"]) {
		names = append(names, str(a["name"]))
	}
	return obj{"id": t["id"], "name": t["name"], "artists": strings.Join(names, ", "), "album": object(t["album"])["name"]}
}
func (c *client) update(collection string, add bool, uris []string) {
	for batch := range slices.Chunk(uris, 40) {
		method := "DELETE"
		if collection == "favorites" {
			if add {
				method = "PUT"
			}
			c.request(method, "/me/library", url.Values{"uris": {strings.Join(batch, ",")}}, nil)
			continue
		}
		body := obj{"uris": batch}
		if add {
			method = "POST"
		} else {
			items := []obj{}
			for _, u := range batch {
				items = append(items, obj{"uri": u})
			}
			body = obj{"items": items}
		}
		c.request(method, "/playlists/"+collection+"/items", nil, body)
	}
}

type journal struct {
	Operation []string `json:"operation"`
	Forward   []string `json:"forward"`
	Reverse   []string `json:"reverse"`
}

func uris(ts []obj) ([]string, map[string]bool) {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range ts {
		u := str(t["uri"])
		if !seen[u] {
			out = append(out, u)
			seen[u] = true
		}
	}
	return out, seen
}
func trackCounts(tracks []obj) map[string]int {
	counts := map[string]int{}
	for _, t := range tracks {
		counts[str(t["uri"])]++
	}
	return counts
}
func selectedCopies(list []string, tracks []obj) []string {
	selected := uriSet(list)
	var out []string
	for _, track := range tracks {
		u := str(track["uri"])
		if selected[u] {
			out = append(out, u)
		}
	}
	return out
}
func missingCopies(wanted []string, existing map[string]int) []string {
	var out []string
	seen := map[string]int{}
	for _, u := range wanted {
		seen[u]++
		if seen[u] > existing[u] {
			out = append(out, u)
		}
	}
	return out
}
func checkRecordedCopies(list []string, current map[string]int) {
	recorded := map[string]int{}
	for _, u := range list {
		recorded[u]++
	}
	for u, count := range recorded {
		if current[u] > count {
			fail("transfer paused: more copies of %s than recorded in the journal", u)
		}
	}
}
func uriSet(list []string) map[string]bool {
	set := map[string]bool{}
	for _, u := range list {
		set[u] = true
	}
	return set
}
func subset(list []string, set map[string]bool, present bool) []string {
	out := []string{}
	for _, v := range list {
		if set[v] == present {
			out = append(out, v)
		}
	}
	return out
}
func (c *client) transfer(cmd, source, dest, track string) obj {
	if source == dest {
		fail("choose two different collections")
	}
	sourceTracks, destTracks := c.tracks(source), c.tracks(dest)
	left, before := uris(sourceTracks)
	right, after := uris(destTracks)
	sourceCounts, destCounts := trackCounts(sourceTracks), trackCounts(destTracks)
	pair := []string{source, dest}
	sort.Strings(pair)
	path := filepath.Join(c.folder, "transfer-"+strings.Join(pair, "-")+".json")
	op := []string{cmd, source, dest}
	if cmd == "move" {
		op = append(op, requiredID(track, "track"))
	}
	j := journal{Operation: op, Forward: left, Reverse: []string{}}
	if readJSON(path, &j) {
		if !slices.Equal(j.Operation, op) {
			fail("resume the interrupted transfer first: %s", strings.Join(j.Operation, " "))
		}
	} else {
		switch cmd {
		case "swap":
			j.Forward = subset(left, after, false)
			j.Reverse = subset(right, before, false)
		case "move":
			u := "spotify:track:" + op[3]
			if !before[u] {
				fail("track absent from source collection")
			}
			j.Forward = []string{u}
		}
		j.Forward = selectedCopies(j.Forward, sourceTracks)
		j.Reverse = selectedCopies(j.Reverse, destTracks)
		if dest == "favorites" {
			for _, u := range j.Forward {
				if sourceCounts[u] > 1 {
					fail("Liked Songs cannot preserve duplicate copies of %s", u)
				}
			}
		}
		if source == "favorites" {
			for _, u := range j.Reverse {
				if destCounts[u] > 1 {
					fail("Liked Songs cannot preserve duplicate copies of %s", u)
				}
			}
		}
		writeJSON(path, j)
	}
	checkRecordedCopies(j.Forward, sourceCounts)
	checkRecordedCopies(j.Reverse, destCounts)
	c.update(dest, true, missingCopies(j.Forward, destCounts))
	c.update(source, true, missingCopies(j.Reverse, sourceCounts))
	c.update(source, false, subset(left, uriSet(j.Forward), true))
	c.update(dest, false, subset(right, uriSet(j.Reverse), true))
	check(os.Remove(path))
	return obj{"source": source, "destination": dest, "forward": len(j.Forward), "reverse": len(j.Reverse)}
}
func newClient(folder, id string) *client {
	return &client{folder: folder, id: id, base: "https://api.spotify.com/v1", http: &http.Client{Timeout: 30 * time.Second}}
}
