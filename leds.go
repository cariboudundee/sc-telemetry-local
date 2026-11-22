package main

// Témoins sur les LED VKB d'après la télémétrie, selon les règles de ledrules.go.
// Option désactivée par défaut, réglée depuis la page. L'outil n'envoie toujours rien au jeu.

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

type vkbState struct {
	vkbMu    sync.Mutex
	vkbOn    bool
	vkbDevs  []string
	vkbErr   string
	vkbLeds  []vkbLed
	vkbIdent vkbIdent
}

func get(m any, path string) any {
	for _, k := range strings.Split(path, ".") {
		mm, ok := m.(map[string]any)
		if !ok {
			return nil
		}
		m = mm[k]
	}
	return m
}

func str(v any) string  { s, _ := v.(string); return s }
func isTrue(v any) bool { b, _ := v.(bool); return b }

// managed : appareils VKB branchés concernés par au moins une règle.
func (a *app) managed() []vkbDevice {
	var out []vkbDevice
	for _, d := range findVKB() {
		for _, l := range a.leds() {
			if l.forDevice(d.Name) {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

// runVKB : boucle de pilotage (4 fois par seconde, renvoi complet toutes les 5 s au cas où un appareil a été rebranché).
func (a *app) runVKB() {
	var devs []vkbDevice
	last := map[string]string{}
	var scan, refresh time.Time
	for range time.Tick(250 * time.Millisecond) {
		a.vkbMu.Lock()
		on := a.vkbOn
		a.vkbMu.Unlock()
		if !on {
			last = map[string]string{}
			continue
		}
		now := time.Now()
		if now.Sub(scan) > 5*time.Second {
			scan, devs = now, a.managed()
			var names []string
			for _, d := range devs {
				names = append(names, d.Name)
			}
			a.vkbMu.Lock()
			a.vkbDevs = names
			a.vkbMu.Unlock()
		}
		force := now.Sub(refresh) > 5*time.Second
		if force {
			refresh = now
		}
		v := a.ledValues()
		errMsg := ""
		for _, d := range devs {
			leds := a.wantedFor(d, v)
			if len(leds) == 0 {
				continue
			}
			key, _ := json.Marshal(leds)
			if !force && last[d.Path] == string(key) {
				continue
			}
			if err := d.setLEDs(leds); err != nil {
				errMsg = d.Name + " : " + err.Error()
				delete(last, d.Path)
				continue
			}
			last[d.Path] = string(key)
		}
		a.vkbMu.Lock()
		a.vkbErr = errMsg
		a.vkbMu.Unlock()
	}
}

func (a *app) vkbHandler(mux *http.ServeMux) {
	mux.HandleFunc("/vkb", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if !localOnly(w, r) {
				return
			}
			on := r.URL.Query().Get("on") == "1"
			a.vkbMu.Lock()
			a.vkbOn = on
			a.vkbMu.Unlock()
			c := loadCfg()
			c.VKB = on
			saveCfg(c)
		}
		a.vkbMu.Lock()
		on, devs, errMsg := a.vkbOn, a.vkbDevs, a.vkbErr
		a.vkbMu.Unlock()
		if !on {
			devs = nil
			for _, d := range a.managed() {
				devs = append(devs, d.Name)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"on": on, "devices": devs, "err": errMsg})
	})
	a.ledRulesHandler(mux)
}