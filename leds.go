package main

// Témoins sur les LED VKB d'après la télémétrie. Option désactivée par défaut, réglée depuis la page.
// L'outil ne pilote que ces LED ; il n'envoie toujours rien au jeu.

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

type vkbState struct {
	vkbMu   sync.Mutex
	vkbOn   bool
	vkbDevs []string
	vkbErr  string
}

var (
	cGreen  = rgb{0, 7, 0}
	cRed    = rgb{7, 0, 0}
	cBlue   = rgb{0, 0, 7}
	cOrange = rgb{7, 3, 0}
)

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
func isTrue(v any) bool   { b, _ := v.(bool); return b }

// annunciator : voyant du jeu → LED (éteinte, couleur selon la gravité, clignotante si le jeu clignote).
func annunciator(s string, normal rgb) ledCfg {
	l := ledCfg{Mode: ledOn, Color: normal}
	switch {
	case s == "" || strings.HasSuffix(s, "_OFF"):
		return ledCfg{Mode: ledOff}
	case strings.Contains(s, "MAJOR"):
		l.Color = cRed
	case strings.Contains(s, "MINOR"):
		l.Color = cOrange
	}
	if strings.Contains(s, "FLASH") || strings.Contains(s, "BLINK") {
		l.Mode = ledSlow
	}
	return l
}

// wanted : LED à afficher pour un appareil, d'après la trame de télémétrie (nil = appareil non géré).
func wanted(name string, frame map[string]any) []ledCfg {
	n := strings.ToUpper(name)
	v, _ := get(frame, "vehicle.state").(map[string]any)
	seated := v != nil && str(get(frame, "actor.state.locomotion")) == "ACTOR_LOCOMOTION_STATE_SEATED"
	off := func(ids ...uint8) []ledCfg {
		var o []ledCfg
		for _, i := range ids {
			o = append(o, ledCfg{ID: i, Mode: ledOff})
		}
		return o
	}
	switch {
	case strings.Contains(n, "GLADIATOR"):
		// 10 : barre de la poignée (couleurs) · 0 : LED bleue. La LED 11 (ministick caméra) n'est pas touchée.
		if !seated {
			return off(10, 0)
		}
		bar := ledCfg{ID: 10, Mode: ledOn, Color: cGreen}
		if strings.Contains(str(get(v, "master_mode.mode")), "NAV") {
			bar.Color = cBlue
		}
		an := func(k string) string { return str(get(v, "annunciators."+k)) }
		if strings.Contains(an("hit"), "MINOR") || strings.Contains(an("hit"), "MAJOR") {
			bar = ledCfg{ID: 10, Mode: ledSlow, Color: cRed}
		}
		if strings.Contains(an("misl"), "MAJOR") {
			bar = ledCfg{ID: 10, Mode: ledFast, Color: cRed}
		}
		dec := ledCfg{ID: 0, Mode: ledOff, Color: cBlue}
		if s, ok := get(v, "flight.settings").(map[string]any); ok && !isTrue(get(s, "ifcs_coupled.enabled")) {
			dec.Mode = ledOn
		}
		return []ledCfg{bar, dec}
	case strings.Contains(n, "S-TECS") || strings.Contains(n, "STECS"):
		// 1 : grande barre (boucliers) · 2 : DNT (train) · 3 : LOCK (quantum) · 0 : ON (centrale)
		if !seated {
			return off(1, 2, 3, 0)
		}
		sh := 0
		if isTrue(get(v, "power.shield_generators")) {
			sh++
		}
		if isTrue(get(v, "power.shield_emitters")) {
			sh++
		}
		shields := ledCfg{ID: 1, Mode: ledOn, Color: [3]rgb{cRed, cOrange, cGreen}[sh]}
		gear := ledCfg{ID: 2, Mode: ledOff}
		switch g := str(get(v, "flight.state.gear")); {
		case strings.HasSuffix(g, "_RETRACTED"):
			gear = ledCfg{ID: 2, Mode: ledOn, Color: cGreen}
		case strings.HasSuffix(g, "_DEPLOYED"):
			gear = ledCfg{ID: 2, Mode: ledOn, Color: cRed}
		case strings.HasSuffix(g, "ING"):
			gear = ledCfg{ID: 2, Mode: ledSlow, Color: cOrange}
		}
		qt := annunciator(str(get(v, "annunciators.qdrv")), cBlue)
		qt.ID = 3
		pw := ledCfg{ID: 0, Mode: ledOff, Color: cBlue}
		if isTrue(get(v, "power.powerplants")) {
			pw.Mode = ledOn
		}
		return []ledCfg{shields, gear, qt, pw}
	}
	return nil
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
			scan, devs = now, findVKB()
			var names []string
			for _, d := range devs {
				if wanted(d.Name, nil) != nil {
					names = append(names, d.Name)
				}
			}
			a.vkbMu.Lock()
			a.vkbDevs = names
			a.vkbMu.Unlock()
		}
		force := now.Sub(refresh) > 5*time.Second
		if force {
			refresh = now
		}
		var frame map[string]any
		if b := a.tel.JSON(); b != nil {
			json.Unmarshal(b, &frame)
		}
		errMsg := ""
		for _, d := range devs {
			leds := wanted(d.Name, frame)
			if leds == nil {
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
		defer a.vkbMu.Unlock()
		devs := a.vkbDevs
		if !a.vkbOn {
			devs = nil
			for _, d := range findVKB() {
				if wanted(d.Name, nil) != nil {
					devs = append(devs, d.Name)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"on": a.vkbOn, "devices": devs, "err": a.vkbErr})
	})
}