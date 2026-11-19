package main

import (
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type command struct {
	name string
	args []string
	opts map[string]string
}
type spec struct {
	usage    string
	min, max int
	flags    string
}

var specs = map[string]spec{
	"login":     {"login", 0, 0, ""},
	"playlists": {"playlists", 0, 0, ""},
	"recent":    {"recent", 0, 0, ""},
	"status":    {"status", 0, 0, ""},
	"devices":   {"devices", 0, 0, ""},
	"search":    {"search QUERY", 1, 1, ""},
	"remove":    {"remove PLAYLIST", 1, 1, ""},
	"create":    {"create NAME", 1, 1, ""},
	"list":      {"list COLLECTION", 1, 1, ""},
	"add":       {"add COLLECTION TRACK", 2, 2, ""},
	"move":      {"move TRACK --from SOURCE --to DESTINATION", 1, 1, "from to"},
	"move-all":  {"move-all --from SOURCE --to DESTINATION", 0, 0, "from to"},
	"swap":      {"swap [COLLECTION] [--from SOURCE]", 0, 1, "from"},
	"play":      {"play [--track TRACK | --playlist PLAYLIST] [--device ID]", 0, 0, "device track playlist"},
	"pause":     {"pause [--device ID]", 0, 0, "device"},
	"previous":  {"previous [--device ID]", 0, 0, "device"},
	"next":      {"next [--device ID]", 0, 0, "device"},
	"volume":    {"volume PERCENT [--device ID]", 1, 1, "device"},
	"seek":      {"seek SECONDS [--device ID]", 1, 1, "device"},
	"shuffle":   {"shuffle on|off [--device ID]", 1, 1, "device"},
	"repeat":    {"repeat off|track|context [--device ID]", 1, 1, "device"},
}

func parse(a []string) command {
	cmd := command{name: a[0], opts: map[string]string{}}
	s, ok := specs[cmd.name]
	if !ok {
		fail("unknown command %q", cmd.name)
	}
	positional := false
	for i := 1; i < len(a); i++ {
		v := a[i]
		if v == "--" && !positional {
			positional = true
			continue
		}
		if positional || !strings.HasPrefix(v, "--") {
			cmd.args = append(cmd.args, v)
			continue
		}
		key, value, inline := strings.Cut(v[2:], "=")
		if !slices.Contains(strings.Fields(s.flags), key) {
			fail("usage: %s", s.usage)
		}
		if !inline {
			i++
			if i >= len(a) || strings.HasPrefix(a[i], "--") {
				fail("missing value for --%s", key)
			}
			value = a[i]
		}
		if value == "" {
			fail("missing value for --%s", key)
		}
		cmd.opts[key] = value
	}
	if len(cmd.args) < s.min || len(cmd.args) > s.max || (cmd.name == "move" || cmd.name == "move-all") && (cmd.opts["from"] == "" || cmd.opts["to"] == "") || cmd.opts["playlist"] != "" && cmd.opts["track"] != "" {
		fail("usage: %s", s.usage)
	}
	return cmd
}
func (c *client) run(cmd command) any {
	a, o, n := cmd.args, cmd.opts, cmd.name
	out := []obj{}
	switch n {
	case "login":
		c.access()
		return obj{"status": "authorized"}
	case "devices":
		for _, d := range rows(c.request("GET", "/me/player/devices", nil, nil)["devices"]) {
			out = append(out, obj{"id": d["id"], "name": d["name"], "active": d["is_active"]})
		}
	case "status":
		s := c.request("GET", "/me/player", nil, nil)
		if len(s) == 0 {
			return obj{"status": "idle"}
		}
		status := "paused"
		if s["is_playing"] == true {
			status = "playing"
		}
		d := object(s["device"])
		return obj{"status": status, "title": object(s["item"])["name"], "device": d["name"], "volume": d["volume_percent"], "progress_ms": s["progress_ms"]}
	case "recent":
		for _, r := range rows(c.request("GET", "/me/player/recently-played", url.Values{"limit": {"50"}}, nil)["items"]) {
			if t := object(r["track"]); t != nil {
				v := summary(t)
				v["played_at"] = r["played_at"]
				out = append(out, v)
			}
		}
	case "playlists":
		for _, p := range c.pages("/me/playlists") {
			out = append(out, obj{"id": p["id"], "name": p["name"]})
		}
	case "search":
		p := c.request("GET", "/search", url.Values{"type": {"track"},
			"q":     {a[0]},
			"limit": {"10"}}, nil)
		for _, t := range rows(object(p["tracks"])["items"]) {
			if str(t["id"]) != "" {
				out = append(out, summary(t))
			}
		}
	case "create":
		p := c.request("POST", "/me/playlists", nil, obj{"name": a[0], "public": false})
		return obj{"name": p["name"], "id": p["id"]}
	case "remove":
		id := c.resolve(a[0])
		if id == "favorites" {
			fail("Liked Songs cannot be removed as a playlist")
		}
		c.request("DELETE", "/me/library", url.Values{"uris": {"spotify:playlist:" + id}}, nil)
		return obj{"id": id, "status": "removed"}
	case "list":
		for _, t := range c.tracks(c.resolve(a[0])) {
			out = append(out, summary(t))
		}
	case "add":
		u := "spotify:track:" + requiredID(a[1], "track")
		dest := c.resolve(a[0])
		_, present := uris(c.tracks(dest))
		if !present[u] {
			c.update(dest, true, []string{u})
		}
		return obj{"destination": dest, "track": u, "added": !present[u]}
	case "move", "move-all", "swap":
		src, dest, track := o["from"], o["to"], ""
		if n == "swap" {
			dest = "norm"
			if len(a) > 0 {
				dest = a[0]
			}
		} else if n == "move" {
			track = a[0]
		}
		dest = c.resolve(dest)
		if src == "" {
			src = "favorites"
			if dest == "favorites" {
				src = "norm"
			}
		}
		return c.transfer(n, c.resolve(src), dest, track)
	default:
		return c.player(cmd)
	}
	return out
}
func (c *client) player(cmd command) obj {
	q := url.Values{}
	if d := cmd.opts["device"]; d != "" {
		q.Set("device_id", d)
	}
	var body any
	method := "PUT"
	switch cmd.name {
	case "play":
		if v := cmd.opts["track"]; v != "" {
			body = obj{"uris": []string{"spotify:track:" + requiredID(v, "track")}}
		} else if v := cmd.opts["playlist"]; v != "" {
			dest := c.resolve(v)
			if dest == "favorites" {
				uris := []string{}
				for _, track := range c.tracks(dest) {
					uris = append(uris, str(track["uri"]))
				}
				if len(uris) == 0 {
					fail("Liked Songs contains no playable tracks")
				}
				body = obj{"uris": uris}
			} else {
				body = obj{"context_uri": "spotify:playlist:" + dest}
			}
		}
	case "next", "previous":
		method = "POST"
	case "volume", "seek":
		v, e := strconv.ParseInt(cmd.args[0], 10, 64)
		if e != nil || v < 0 {
			fail("expected a nonnegative integer")
		}
		if cmd.name == "volume" {
			if v > 100 {
				fail("volume must be 0 to 100")
			}
			q.Set("volume_percent", strconv.FormatInt(v, 10))
		} else {
			if v > 9223372036854775 {
				fail("seek position too large")
			}
			q.Set("position_ms", strconv.FormatInt(v*1000, 10))
		}
	case "shuffle":
		v := cmd.args[0]
		if v != "on" && v != "off" {
			fail("shuffle accepts on or off")
		}
		q.Set("state", strconv.FormatBool(v == "on"))
	case "repeat":
		v := cmd.args[0]
		if !slices.Contains([]string{"track", "off", "context"}, v) {
			fail("repeat accepts track, context or off")
		}
		q.Set("state", v)
	}
	c.request(method, "/me/player/"+cmd.name, q, body)
	return obj{"command": cmd.name, "accepted": true}
}

func (c *client) idlePlaybackTarget(requested string) string {
	devices := rows(c.request("GET", "/me/player/devices", nil, nil)["devices"])
	var available []string
	for _, device := range devices {
		id := str(device["id"])
		if id == "" || device["is_restricted"] == true || (requested != "" && requested != id) {
			continue
		}
		if device["is_active"] == true {
			return ""
		}
		available = append(available, id)
	}
	if len(available) == 1 {
		return available[0]
	}
	if len(available) > 1 {
		fail("Choose a playback device with devices, then play --device ID.")
	}
	return ""
}
