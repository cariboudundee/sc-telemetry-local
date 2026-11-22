package main

// Accès depuis le réseau local (téléphone, tablette) : activé depuis la page, mémorisé, avec QR code.
// La page reste en lecture seule pour ces appareils (les actions sont réservées au PC, voir localOnly).

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	qrcode "github.com/skip2/go-qrcode"
)

type config struct {
	LAN bool `json:"lan"`
}

func cfgPath() string { return filepath.Join(os.Getenv("APPDATA"), "SCTelemetryLocal", "config.json") }

func loadCfg() config {
	var c config
	if b, err := os.ReadFile(cfgPath()); err == nil {
		json.Unmarshal(b, &c)
	}
	return c
}

func saveCfg(c config) {
	os.MkdirAll(filepath.Dir(cfgPath()), 0o755)
	b, _ := json.MarshalIndent(c, "", " ")
	os.WriteFile(cfgPath(), b, 0o644)
}

type lanState struct {
	mux     *http.ServeMux
	port    int
	lanOn   bool
	lanMu   sync.Mutex
	lanLn   []net.Listener
	lanURLs []string
	lanErr  string
}

// startLAN écoute en plus sur chaque adresse privée du PC (le 127.0.0.1 reste actif à côté).
func (a *app) startLAN() {
	a.lanMu.Lock()
	defer a.lanMu.Unlock()
	if len(a.lanLn) > 0 {
		return
	}
	a.lanErr = ""
	for _, ip := range lanAddresses() {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ip, a.port))
		if err != nil {
			a.lanErr = err.Error()
			continue
		}
		a.lanLn = append(a.lanLn, ln)
		u := fmt.Sprintf("http://%s:%d", ip, a.port)
		a.lanURLs = append(a.lanURLs, u)
		fmt.Println("Depuis un téléphone :", u)
		go http.Serve(ln, a.mux)
	}
	a.lanOn = true
}

func (a *app) stopLAN() {
	a.lanMu.Lock()
	defer a.lanMu.Unlock()
	for _, ln := range a.lanLn {
		ln.Close()
	}
	a.lanLn, a.lanURLs, a.lanOn = nil, nil, false
	fmt.Println("Accès réseau local coupé.")
}

func (a *app) lanHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/lan", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if !localOnly(w, r) {
				return
			}
			on := r.URL.Query().Get("on") == "1"
			if on {
				a.startLAN()
			} else {
				a.stopLAN()
			}
			saveCfg(config{LAN: on})
		}
		a.lanMu.Lock()
		defer a.lanMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"on": a.lanOn, "urls": a.lanURLs, "err": a.lanErr})
	})
	// QR code, uniquement pour une adresse que l'outil sert réellement
	mux.HandleFunc("/qr", func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query().Get("u")
		a.lanMu.Lock()
		ok := false
		for _, x := range a.lanURLs {
			ok = ok || x == u
		}
		a.lanMu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		png, err := qrcode.Encode(u, qrcode.Medium, 220)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	})
}
