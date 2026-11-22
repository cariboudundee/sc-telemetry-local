package main

// Serveur web local : page du tableau de bord + flux temps réel (Server-Sent Events).
// Écoute sur 127.0.0.1 par défaut ; sur le réseau local seulement avec -lan.

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

//go:embed web
var webFS embed.FS

type snapshot struct {
	Version   string          `json:"v"`
	Branch    string          `json:"branch"`
	Connected bool            `json:"connected"`
	Frame     json.RawMessage `json:"frame"`
	Game      json.RawMessage `json:"game"`
}

func (a *app) snapshot() []byte {
	s := snapshot{Version: version, Branch: a.in.Branch}
	a.tel.mu.RLock()
	s.Connected = a.tel.Connected
	a.tel.mu.RUnlock()
	if b := a.tel.JSON(); b != nil {
		s.Frame = b
	} else {
		s.Frame = json.RawMessage("null")
	}
	a.gi.mu.Lock()
	s.Game, _ = json.Marshal(a.gi)
	a.gi.mu.Unlock()
	b, _ := json.Marshal(s)
	return b
}

type app struct {
	in  *Install
	tel *Telemetry
	gi  *GameInfo
	lanState
	vkbState
	vpcState
}

func (a *app) serve(addr string) error {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/setup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(a.setupState())
	})
	mux.HandleFunc("/setup/env", func(w http.ResponseWriter, r *http.Request) {
		if !localOnly(w, r) {
			return
		}
		if err := enableEnv(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(a.snapshot())
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "flux non pris en charge", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		var last string
		for {
			select {
			case <-r.Context().Done():
				return
			case <-tick.C:
				b := a.snapshot()
				if string(b) == last {
					continue
				}
				last = string(b)
				fmt.Fprintf(w, "data: %s\n\n", b)
				fl.Flush()
			}
		}
	})
	a.lanHandlers(mux)
	a.firewallHandler(mux)
	a.vkbHandler(mux)
	autostartHandler(mux)
	go a.runVKB()
	a.vpcHandler(mux)
	go a.runVPC()
	a.mux = mux
	if a.lanOn {
		a.lanOn = false
		a.startLAN()
	}
	var err error
	for i := 0; i < 20; i++ { // après un redémarrage, l'ancienne instance libère le port
		if err = http.ListenAndServe(addr, mux); !strings.Contains(fmt.Sprint(err), "bind") {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// lanAddresses : adresses IPv4 privées du PC, pour l'accès depuis un téléphone.
func lanAddresses() []string {
	var out []string
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 || virtualIf(i.Name) {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && n.IP.IsPrivate() {
				out = append(out, n.IP.String())
			}
		}
	}
	return out
}

func openBrowser(url string) { exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() }




