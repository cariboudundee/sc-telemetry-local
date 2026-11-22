package main

// Règles des LED VKB, réglables depuis la page « LED VKB » : pour chaque LED d'un appareil, une liste
// de règles lues dans l'ordre ; la première dont toutes les conditions sont vraies donne le mode et la
// couleur. Aucune règle vraie : LED éteinte. Une LED sans règle n'est pas touchée (fonction d'origine).
// Les conditions portent sur les champs de la télémétrie sous leur nom d'origine (comme pour Virpil).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type ledCond struct {
	Arg   string `json:"arg"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

type ledRule struct {
	Conds    []ledCond `json:"conds"`
	Mode     string    `json:"mode"` // off, on, slow, fast, ultra
	Color    string    `json:"color"`
	Disabled bool      `json:"disabled,omitempty"`
}

type vkbLed struct {
	Device string    `json:"device"` // partie du nom de l'appareil, ex. « Gladiator »
	LED    int       `json:"led"`
	Label  string    `json:"label"`
	Rules  []ledRule `json:"rules"`
}

func cond(arg, op, v string) ledCond { return ledCond{arg, op, v} }
func rule(mode, color string, cs ...ledCond) ledRule {
	return ledRule{Conds: cs, Mode: mode, Color: color}
}

// defaultLeds : réglage d'origine (celui validé sur un Gladiator EVO R et un S-TECS Mini Plus).
func defaultLeds() []vkbLed {
	off := rule("off", "#000000", cond("seated", "==", "0"))
	return []vkbLed{
		{"Gladiator", 10, "Barre de la poignée", []ledRule{off,
			rule("fast", "#ff0000", cond("annunciators.misl", "==", "MAJOR")),
			rule("slow", "#ff0000", cond("annunciators.hit", "==", "MAJOR")),
			rule("slow", "#ff0000", cond("annunciators.hit", "==", "MINOR")),
			rule("on", "#0000ff", cond("master_mode.mode", "==", "NAVIGATION")),
			rule("on", "#00ff00")}},
		{"Gladiator", 0, "LED bleue", []ledRule{off,
			rule("on", "#0000ff", cond("flight.settings.ifcs_coupled.enabled", "==", "0"))}},
		{"S-TECS", 1, "Grande barre", []ledRule{off,
			rule("on", "#00ff00", cond("power.shield_generators", "==", "1"), cond("power.shield_emitters", "==", "1")),
			rule("on", "#ff6600", cond("power.shield_generators", "==", "1")),
			rule("on", "#ff6600", cond("power.shield_emitters", "==", "1")),
			rule("on", "#ff0000")}},
		{"S-TECS", 2, "DNT", []ledRule{off,
			rule("on", "#00ff00", cond("flight.state.gear", "==", "RETRACTED")),
			rule("on", "#ff0000", cond("flight.state.gear", "==", "DEPLOYED")),
			rule("slow", "#ff6600", cond("flight.state.gear", "==", "DEPLOYING")),
			rule("slow", "#ff6600", cond("flight.state.gear", "==", "RETRACTING"))}},
		{"S-TECS", 3, "LOCK", []ledRule{off,
			rule("on", "#ff0000", cond("annunciators.qdrv", "==", "MAJOR")),
			rule("on", "#ff6600", cond("annunciators.qdrv", "==", "MINOR")),
			rule("on", "#0000ff", cond("annunciators.qdrv", "==", "NOMINAL"))}},
		{"S-TECS", 0, "ON", []ledRule{off,
			rule("on", "#0000ff", cond("power.powerplants", "==", "1"))}},
	}
}

// ledValues : valeurs (nombres) et noms (énumérations) de la trame, plus « seated ».
type ledValues struct {
	nums  map[string]any
	names map[string]string
}

func (a *app) ledValues() ledValues {
	v := ledValues{nums: map[string]any{}, names: enumNames(jsonMap(a.tel.JSONFull(false)))}
	flatten("", jsonMap(a.tel.JSONFull(true)), v.nums)
	f := jsonMap(a.tel.JSON())
	seated := get(f, "vehicle.state") != nil && str(get(f, "actor.state.locomotion")) == "ACTOR_LOCOMOTION_STATE_SEATED"
	v.nums["seated"] = b01(seated)
	return v
}

func toFloat(x any) (float64, bool) {
	switch y := x.(type) {
	case float64:
		return y, true
	case int:
		return float64(y), true
	}
	return 0, false
}

func (c ledCond) match(v ledValues) bool {
	x, ok := v.nums[c.Arg]
	if !ok {
		return false
	}
	want := strings.TrimSpace(c.Value)
	if w, err := strconv.ParseFloat(want, 64); err == nil {
		f, ok := toFloat(x)
		if !ok {
			return false
		}
		switch c.Op {
		case "!=":
			return f != w
		case ">":
			return f > w
		case "<":
			return f < w
		case ">=":
			return f >= w
		case "<=":
			return f <= w
		}
		return f == w
	}
	name := v.names[c.Arg]
	if name == "" {
		name = fmt.Sprint(x)
	}
	N, W := strings.ToUpper(name), strings.ToUpper(want)
	eq := N == W || strings.HasSuffix(N, "_"+W)
	if c.Op == "!=" {
		return !eq
	}
	return eq
}

var modes = map[string]uint8{"off": ledOff, "on": ledOn, "slow": ledSlow, "fast": ledFast, "ultra": ledUltra}

func hexColor(s string) rgb {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return rgb{7, 7, 7}
	}
	var out rgb
	for i := 0; i < 3; i++ {
		n, _ := strconv.ParseUint(s[2*i:2*i+2], 16, 8)
		out[i] = uint8((n*7 + 127) / 255)
	}
	return out
}

// eval : première règle vraie d'une LED (index de la règle, -1 si aucune).
func (l vkbLed) eval(v ledValues) (ledCfg, int) {
	for i, r := range l.Rules {
		if r.Disabled {
			continue
		}
		ok := true
		for _, c := range r.Conds {
			ok = ok && c.match(v)
		}
		if ok {
			return ledCfg{ID: uint8(l.LED), Mode: modes[r.Mode], Color: hexColor(r.Color)}, i
		}
	}
	return ledCfg{ID: uint8(l.LED), Mode: ledOff}, -1
}

func (l vkbLed) forDevice(name string) bool {
	return l.Device != "" && strings.Contains(strings.ToUpper(name), strings.ToUpper(l.Device))
}

func (a *app) leds() []vkbLed {
	a.vkbMu.Lock()
	defer a.vkbMu.Unlock()
	if a.vkbLeds == nil {
		if c := loadCfg(); c.VKBLeds != nil {
			a.vkbLeds = c.VKBLeds
		} else {
			a.vkbLeds = defaultLeds()
		}
	}
	return a.vkbLeds
}

// wantedFor : LED à envoyer à un appareil (au plus 12), identification comprise.
func (a *app) wantedFor(d vkbDevice, v ledValues) []ledCfg {
	var out []ledCfg
	seen := map[int]bool{}
	for _, l := range a.leds() {
		if !l.forDevice(d.Name) || seen[l.LED] || len(out) == 12 {
			continue
		}
		seen[l.LED] = true
		cfg, _ := l.eval(v)
		out = append(out, cfg)
	}
	a.vkbMu.Lock()
	id := a.vkbIdent
	a.vkbMu.Unlock()
	if id.path == d.Path && time.Now().Before(id.until) {
		blink := ledCfg{ID: uint8(id.led), Mode: ledFast, Color: rgb{7, 7, 7}}
		for i := range out {
			if int(out[i].ID) == id.led {
				out[i] = blink
				return out
			}
		}
		if len(out) < 12 {
			out = append(out, blink)
		}
	}
	return out
}

type vkbIdent struct {
	path  string
	led   int
	until time.Time
}

func (a *app) ledRulesHandler(mux *http.ServeMux) {
	mux.HandleFunc("/vkb/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if !localOnly(w, r) {
				return
			}
			var leds []vkbLed
			if r.URL.Query().Get("reset") == "1" {
				leds = defaultLeds()
			} else if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&leds); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			a.vkbMu.Lock()
			a.vkbLeds = leds
			a.vkbMu.Unlock()
			c := loadCfg()
			c.VKBLeds = leds
			saveCfg(c)
		}
		v := a.ledValues()
		leds := a.leds()
		active := make([]int, len(leds))
		for i, l := range leds {
			_, active[i] = l.eval(v)
		}
		var devs []string
		for _, d := range findVKB() {
			devs = append(devs, d.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"leds": leds, "active": active, "devices": devs, "values": v.nums, "names": v.names})
	})
	// identification : la LED clignote en blanc 4 s
	mux.HandleFunc("/vkb/identify", func(w http.ResponseWriter, r *http.Request) {
		if !localOnly(w, r) {
			return
		}
		dev, led := r.URL.Query().Get("device"), r.URL.Query().Get("led")
		n, err := strconv.Atoi(led)
		if err != nil || n < 0 || n > 63 {
			http.Error(w, "numéro de LED invalide", 400)
			return
		}
		for _, d := range findVKB() {
			if (vkbLed{Device: dev}).forDevice(d.Name) {
				a.vkbMu.Lock()
				a.vkbIdent = vkbIdent{d.Path, n, time.Now().Add(4 * time.Second)}
				a.vkbMu.Unlock()
				w.Write([]byte("ok"))
				return
			}
		}
		http.Error(w, "appareil non trouvé", 404)
	})
}