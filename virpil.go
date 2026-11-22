package main

// Pont vers VPC Link Tool (Virpil) : l'outil se fait passer pour le script DCS de Virpil et envoie
// l'état du vaisseau en JSON, en UDP sur 127.0.0.1:4123, 10 fois par seconde. Les LED se règlent
// ensuite dans VPC Link Tool comme pour DCS. Champs de la télémétrie sous leur nom d'origine + clés DCS standard.

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
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

// flatten : champs de la télémétrie sous leur nom d'origine (chemin à points, sans le préfixe
// « vehicle.state. »), booléens en 0/1, nombres tels quels, énumérations en nombres.
func flatten(prefix string, v any, out map[string]any) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			flatten(prefix+k+".", e, out)
		}
	case []any:
		for i, e := range x {
			flatten(fmt.Sprintf("%s%d.", prefix, i), e, out)
		}
	case nil:
	default:
		k := strings.TrimPrefix(strings.TrimSuffix(prefix, "."), "vehicle.state.")
		switch y := x.(type) {
		case bool:
			out[k] = b01(y)
		case string:
			if f, err := strconv.ParseFloat(y, 64); err == nil {
				out[k] = f
			} else {
				out[k] = y
			}
		default:
			out[k] = x
		}
	}
}

// vpcData : valeurs envoyées à VPC Link Tool. full = trame complète (énumérations en nombres),
// frame = trame habituelle (pour les clés DCS). known = clés déjà vues, renvoyées à 0 si elles disparaissent.
func vpcData(full, frame map[string]any, known map[string]bool) map[string]any {
	d := map[string]any{}
	flatten("", full, d)
	for k := range known {
		if _, ok := d[k]; !ok {
			d[k] = 0
		}
	}
	for k, x := range d {
		if _, isStr := x.(string); !isStr && known != nil {
			known[k] = true
		}
	}
	for k, x := range dcsKeys(frame) {
		d[k] = x
	}
	return d
}

// dcsKeys : table de conversion Star Citizen → clés du script DCS de Virpil.
func dcsKeys(frame map[string]any) map[string]any {
	d := map[string]any{}
	v, _ := get(frame, "vehicle.state").(map[string]any)
	if v == nil || str(get(frame, "actor.state.locomotion")) != "ACTOR_LOCOMOTION_STATE_SEATED" {
		d["export_allowed"] = 0
		return d
	}
	d["export_allowed"] = 1
	name := str(v["class_name"])
	if name == "" {
		name = str(v["name"])
	}
	d["aircraftType"] = name
	ins, _ := get(v, "flight.instruments").(map[string]any)
	g := str(get(v, "flight.state.gear"))
	gs, gv := 0, 0
	switch {
	case strings.HasSuffix(g, "_DEPLOYED"):
		gs, gv = 1, 100
	case strings.HasSuffix(g, "ING"):
		gs, gv = 1, 50
	}
	d["mech_Gear_Status"], d["mech_Gear_Value"] = gs, gv
	master := 0
	if an, ok := v["annunciators"].(map[string]any); ok {
		for _, s := range an {
			if strings.Contains(str(s), "MAJOR") {
				master = 1
			}
		}
	}
	d["mcp_MasterWarning"] = master
	d["mcp_AutopilotOn"] = b01(isTrue(get(v, "flight.settings.ifcs_cruise_mode.active")))
	d["plane_IndicatedAirSpeed"] = int(num(ins["speed"]))
	d["plane_TrueAirSpeed"] = int(num(ins["speed"]))
	d["plane_AltitudeAboveSeaLevel"] = int(num(ins["altitude"]))
	d["plane_RadarAltimeter"] = int(num(ins["radar_altitude"]))
	d["plane_AltitudeAboveGroundLevel"] = int(num(ins["radar_altitude"]))
	d["plane_VerticalVelocity"] = int(num(ins["vertical_speed_indicator"]))
	d["plane_CurrentCourse"] = int(num(ins["heading"]))
	d["plane_flare"], d["plane_chaff"] = 0, 0
	if cms, ok := get(frame, "weapons.countermeasures").([]any); ok {
		for _, c := range cms {
			if strings.Contains(str(get(c, "type")), "DECOY") {
				d["plane_flare"] = int(num(get(c, "amount")))
			} else {
				d["plane_chaff"] = int(num(get(c, "amount")))
			}
		}
	}
	return d
}

// enumNames : pour l'affichage dans la page, nom de chaque valeur d'énumération envoyée en nombre.
func enumNames(named map[string]any) map[string]string {
	d := map[string]any{}
	flatten("", named, d)
	out := map[string]string{}
	for k, x := range d {
		if s, ok := x.(string); ok && strings.Contains(s, "_") && strings.ToUpper(s) == s {
			out[k] = s
		}
	}
	return out
}

func jsonMap(b []byte) map[string]any {
	var m map[string]any
	if b != nil {
		json.Unmarshal(b, &m)
	}
	return m
}
func (a *app) runVPC() {
	var conn net.Conn
	known := map[string]bool{}
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
		b, _ := json.Marshal(vpcData(jsonMap(a.tel.JSONFull(true)), jsonMap(a.tel.JSON()), known))
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
		data := vpcData(jsonMap(a.tel.JSONFull(true)), jsonMap(a.tel.JSON()), nil)
		names := enumNames(jsonMap(a.tel.JSONFull(false)))
		a.vpcMu.Lock()
		defer a.vpcMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"on": a.vpcOn, "addr": vpcAddr, "sent": a.vpcSent, "err": a.vpcErr,
			"linktool": running("VPC_Link_Tool.exe", "VPC Link Tool.exe", "VPCLinkTool.exe"), "data": data, "enums": names})
	})
}