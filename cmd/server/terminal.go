// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package main

// PTY-backed shell over a WebSocket for the in-browser terminal. Full shell
// access, so superuser-only; the auth token arrives as the ?token= query param
// since WebSockets cannot send the Authorization header.

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

var terminalUpgrader = websocket.Upgrader{
	// Same-origin only (defense in depth on the superuser-token gate); empty Origin (non-browser clients) is allowed.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		return err == nil && u.Host == r.Host
	},
}

// termResizeMsg is sent from the browser when the terminal is resized.
type termResizeMsg struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// terminalWSHandler validates a superuser token, then bridges a PTY shell over the WebSocket until either side closes.
func terminalWSHandler(app *pocketbase.PocketBase) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		token := e.Request.URL.Query().Get("token")
		rec, err := app.FindAuthRecordByToken(token, core.TokenTypeAuth)
		if err != nil || rec == nil || rec.Collection().Name != core.CollectionNameSuperusers {
			e.Response.WriteHeader(http.StatusUnauthorized)
			return nil
		}

		conn, err := terminalUpgrader.Upgrade(e.Response, e.Request, nil)
		if err != nil {
			return err
		}
		defer conn.Close()

		ts := resolveTerminalSession()
		cmd := exec.Command(ts.shell, "-l")
		cmd.Env = ts.env
		cmd.Dir = ts.dir
		if ts.cred != nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: ts.cred}
		}

		ptmx, err := pty.Start(cmd)
		if err != nil {
			log.Printf("[terminal] failed to start PTY: %v", err)
			_ = conn.WriteMessage(websocket.TextMessage, []byte("Error: failed to start shell: "+err.Error()))
			return nil
		}
		defer func() {
			_ = ptmx.Close()
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}()

		_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80})

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := make([]byte, 4096)
			for {
				n, readErr := ptmx.Read(buf)
				if n > 0 {
					if conn.WriteMessage(websocket.BinaryMessage, buf[:n]) != nil {
						return
					}
				}
				if readErr != nil {
					return
				}
			}
		}()

		// WebSocket -> PTY stdin (resize messages handled inline).
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				msgType, data, readErr := conn.ReadMessage()
				if readErr != nil {
					return
				}
				if msgType == websocket.TextMessage {
					var resize termResizeMsg
					if json.Unmarshal(data, &resize) == nil && resize.Type == "resize" {
						_ = pty.Setsize(ptmx, &pty.Winsize{Rows: resize.Rows, Cols: resize.Cols})
						continue
					}
				}
				if _, writeErr := ptmx.Write(data); writeErr != nil {
					if !errors.Is(writeErr, io.EOF) {
						log.Printf("[terminal] PTY write error: %v", writeErr)
					}
					return
				}
			}
		}()

		wg.Wait()
		return nil
	}
}

// terminalSession is the resolved shell, environment, and optional privilege drop for an in-browser terminal.
type terminalSession struct {
	shell string
	env   []string
	dir   string
	cred  *syscall.Credential // nil = run as the current (service) user
}

// resolveTerminalSession runs the operator's own login shell, as that user, in
// their home, dropping the service's root privileges to the operator. Operator
// resolution order: TALA_TERMINAL_USER, TALA_OPERATOR, first regular account in
// /etc/passwd, then the current user. Set TALA_TERMINAL_USER=root for root.
func resolveTerminalSession() terminalSession {
	name := strings.TrimSpace(os.Getenv("TALA_TERMINAL_USER"))
	if name == "" {
		name = strings.TrimSpace(os.Getenv("TALA_OPERATOR"))
	}
	var u *user.User
	if name != "" {
		u, _ = user.Lookup(name)
	}
	if u == nil {
		if n := firstRegularUsername(); n != "" {
			u, _ = user.Lookup(n)
		}
	}
	if u == nil {
		u, _ = user.Current()
	}

	shell := shellForUser(u)
	env := []string{
		"TERM=xterm-256color",
		"HOME=" + u.HomeDir,
		"USER=" + u.Username,
		"LOGNAME=" + u.Username,
		"SHELL=" + shell,
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
	// Carry locale/timezone through so the login session renders correctly.
	for _, k := range []string{"LANG", "LC_ALL", "LC_CTYPE", "TZ"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}

	ts := terminalSession{shell: shell, env: env, dir: u.HomeDir}
	if uid, err := strconv.Atoi(u.Uid); err == nil && uid != os.Geteuid() {
		gid, _ := strconv.Atoi(u.Gid)
		ts.cred = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: supplementaryGIDs(u)}
	}
	if ts.dir == "" {
		ts.dir = "/"
	}
	return ts
}

// shellForUser returns the account's login shell, falling back to zsh, $SHELL, then /bin/bash.
func shellForUser(u *user.User) string {
	if sh := passwdShell(u.Username); sh != "" {
		return sh
	}
	if _, err := os.Stat(filepath.Join(u.HomeDir, ".zshrc")); err == nil {
		if z, err := exec.LookPath("zsh"); err == nil {
			return z
		}
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/bash"
}

func passwdShell(username string) string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Split(line, ":")
		if len(f) >= 7 && f[0] == username {
			return f[6]
		}
	}
	return ""
}

// firstRegularUsername returns the first human account (UID 1000..64999 with a real login shell and an existing home).
func firstRegularUsername() string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Split(line, ":")
		if len(f) < 7 {
			continue
		}
		uid, err := strconv.Atoi(f[2])
		if err != nil || uid < 1000 || uid >= 65000 {
			continue
		}
		if f[6] == "" || strings.HasSuffix(f[6], "/nologin") || strings.HasSuffix(f[6], "/false") {
			continue
		}
		if st, err := os.Stat(f[5]); err != nil || !st.IsDir() {
			continue
		}
		return f[0]
	}
	return ""
}

// supplementaryGIDs returns the account's secondary group IDs so the dropped shell keeps its group memberships.
func supplementaryGIDs(u *user.User) []uint32 {
	ids, err := u.GroupIds()
	if err != nil {
		return nil
	}
	out := make([]uint32, 0, len(ids))
	for _, g := range ids {
		if n, err := strconv.Atoi(g); err == nil {
			out = append(out, uint32(n))
		}
	}
	return out
}
