package main

// Pont vers VPC Link Tool (Virpil) : l'outil se fait passer pour le script DCS de Virpil et envoie
// l'état du vaisseau en JSON, en UDP sur 127.0.0.1:4123, 10 fois par seconde. Les LED se règlent
// ensuite dans VPC Link Tool comme pour DCS. Clés DCS standard (table de conversion) + clés sc_*.

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const vpcAddr = "127.0.0.1:4123"

type vpcState struct {
	vpcMu   sync.Mutex
	vpcOn   bool
	vpcSent int
	vpcErr  string
}

func b01(b bool) int {
	if b {
		return 1
	}
	return 0
}

func num(v any) float64 { f, _ := v.(float64); return f }

// vpcData : table de conversion Star Citizen → clés du script DCS de Virpil, plus clés sc_*.
func vpcData(frame map[string]any) map[string]any {
	d := map[string]any{}
	v, _ := get(frame, "vehicle.state").(map[string]any)
	seated := v != nil && str(get(frame, "actor.state.locomotion")) == "ACTOR_LOCOMOTION_STATE_SEATED"
	d["sc_seated"] = b01(seated)
	if !seated {
		d["export_allowed"] = 0
		return d
	}
	name := str(v["class_name"])
	if name == "" {
		name = str(v["name"])
	}
	d["aircraftType"] = name
	ins, _ := get(v, "flight.instruments").(map[string]any)

	// train : 0 rentré, 1 sorti, 0.5 en mouvement (comme LoGetMechInfo().gear de DCS)
	g := str(get(v, "flight.state.gear"))
	gs, gv := 0, 0
	switch {
	case strings.HasSuffix(g, "_DEPLOYED"):
		gs, gv = 1, 100
	case strings.HasSuffix(g, "ING"):
		gs, gv = 1, 50
	}
	d["mech_Gear_Status"], d["mech_Gear_Value"] = gs, gv
	d["sc_gear"] = gv

	an := func(k string) string { return str(get(v, "annunciators."+k)) }
	level := func(s string) int { // 0 éteint, 1 normal, 2 attention, 3 alerte
		switch {
		case s == "" || strings.HasSuffix(s, "_OFF"):
			return 0
		case strings.Contains(s, "MAJOR"):
			return 3
		case strings.Contains(s, "MINOR"):
			return 2
		}
		return 1
	}
	master := 0
	for _, k := range []string{"pwr", "wpn", "thr", "shld", "cool", "qdrv", "radr", "prox", "hit", "misl"} {
		l := level(an(k))
		d["sc_ann_"+k] = l
		if l == 3 {
			master = 1
		}
	}
	d["mcp_MasterWarning"] = master
	d["sc_hit"] = b01(level(an("hit")) >= 2)
	d["sc_missile"] = b01(level(an("misl")) == 3)

	sh := b01(isTrue(get(v, "power.shield_generators"))) + b01(isTrue(get(v, "power.shield_emitters")))
	d["sc_shields"] = sh // 0 off, 1 partiel, 2 OK
	for _, k := range []string{"powerplants", "engines", "weapons", "coolers", "life_support", "sensors", "gravity_generators"} {
		d["sc_power_"+k] = b01(isTrue(get(v, "power."+k)))
	}
	set, _ := get(v, "flight.settings").(map[string]any)
	d["sc_decoupled"] = b01(set != nil && !isTrue(get(set, "ifcs_coupled.enabled")))
	for _, k := range []string{"ifcs_esp", "ifcs_g_safe", "ifcs_cruise_mode", "ifcs_speed_limiter", "ifcs_proximity_assist"} {
		d["sc_"+strings.TrimPrefix(k, "ifcs_")] = b01(isTrue(get(set, k+".active")))
	}
	d["mcp_AutopilotOn"] = b01(isTrue(get(set, "ifcs_cruise_mode.active")))
	d["sc_lights"] = b01(isTrue(v["lights_enabled"]))
	d["sc_vtol"] = b01(strings.HasSuffix(str(get(v, "flight.state.vtol")), "_DEPLOYED"))
	mm := str(get(v, "master_mode.mode"))
	d["sc_nav"] = b01(strings.Contains(mm, "NAV"))
	d["sc_scm"] = b01(strings.Contains(mm, "SCM"))
	d["sc_gimbals_locked"] = b01(isTrue(get(frame, "weapons.settings.gimbals_locked")))

	// instruments (mêmes noms que le script DCS)
	d["plane_IndicatedAirSpeed"] = int(num(ins["speed"]))
	d["plane_TrueAirSpeed"] = int(num(ins["speed"]))
	d["plane_AltitudeAboveSeaLevel"] = int(num(ins["altitude"]))
	d["plane_RadarAltimeter"] = int(num(ins["radar_altitude"]))
	d["plane_AltitudeAboveGroundLevel"] = int(num(ins["radar_altitude"]))
	d["plane_VerticalVelocity"] = int(num(ins["vertical_speed_indicator"]))
	d["plane_CurrentCourse"] = int(num(ins["heading"]))
	d["sc_hydrogen"] = int(num(ins["hydrogen_fuel"]) * 100)
	d["sc_quantum_fuel"] = int(num(ins["quantum_fuel"]) * 100)
	d["sc_health"] = int(num(v["health"]) * 100)
	d["sc_shield_level"] = int(num(get(v, "shields.level")) * 100)

	// contre-mesures : leurres = flare, bruit = chaff
	if cms, ok := get(frame, "weapons.countermeasures").([]any); ok {
		for _, c := range cms {
			t, n := str(get(c, "type")), int(num(get(c, "amount")))
			if strings.Contains(t, "DECOY") {
				d["plane_flare"] = n
			} else {
				d["plane_chaff"] = n
			}
		}
	}
	return d
}

func (a *app) runVPC() {
	var conn net.Conn
	for range time.Tick(100 * time.Millisecond) {
		a.vpcMu.Lock()
		on := a.vpcOn
		a.vpcMu.Unlock()
		if !on {
			if conn != nil {
				conn.Close()
				conn = nil
			}
			continue
		}
		if conn == nil {
			var err error
			if conn, err = net.Dial("udp", vpcAddr); err != nil {
				a.vpcMu.Lock()
				a.vpcErr = err.Error()
				a.vpcMu.Unlock()
				continue
			}
		}
		var frame map[string]any
		if b := a.tel.JSON(); b != nil {
			json.Unmarshal(b, &frame)
		}
		b, _ := json.Marshal(vpcData(frame))
		_, err := conn.Write(b)
		a.vpcMu.Lock()
		if err != nil {
			a.vpcErr = err.Error()
		} else {
			a.vpcErr = ""
			a.vpcSent++
		}
		a.vpcMu.Unlock()
	}
}

func (a *app) vpcHandler(mux *http.ServeMux) {
	mux.HandleFunc("/vpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if !localOnly(w, r) {
				return
			}
			on := r.URL.Query().Get("on") == "1"
			a.vpcMu.Lock()
			a.vpcOn = on
			a.vpcMu.Unlock()
			c := loadCfg()
			c.VPC = on
			saveCfg(c)
		}
		var frame map[string]any
		if b := a.tel.JSON(); b != nil {
			json.Unmarshal(b, &frame)
		}
		a.vpcMu.Lock()
		defer a.vpcMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"on": a.vpcOn, "addr": vpcAddr, "sent": a.vpcSent, "err": a.vpcErr,
			"linktool": running("VPC_Link_Tool.exe", "VPC Link Tool.exe", "VPCLinkTool.exe"), "data": vpcData(frame)})
	})
}