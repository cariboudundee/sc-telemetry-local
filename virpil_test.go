package main

import (
	"encoding/json"
	"testing"
)

const sampleFrame = `{"vehicle":{"state":{"class_name":"GAMA_Hyun","lights_enabled":true,
"annunciators":{"pwr":"ANNUNCIATOR_STATE_NOMINAL","hit":"ANNUNCIATOR_STATE_MINOR","misl":"ANNUNCIATOR_STATE_MAJOR","qdrv":"ANNUNCIATOR_STATE_OFF"},
"flight":{"state":{"gear":"VEHICLE_COMPONENT_DEPLOYMENT_STATE_DEPLOYED","vtol":"VEHICLE_COMPONENT_DEPLOYMENT_STATE_DEPLOYED"},
"settings":{"ifcs_coupled":{},"ifcs_cruise_mode":{"enabled":true,"active":true}},
"instruments":{"speed":12.6,"altitude":1093.5,"radar_altitude":20.2,"heading":174.2,"hydrogen_fuel":0.95}},
"power":{"powerplants":true,"shield_generators":true},"shields":{"level":1},"health":1,
"master_mode":{"mode":"VEHICLE_MASTER_MODE_NAVIGATION"}}},
"weapons":{"countermeasures":[{"type":"COUNTERMEASURE_TYPE_DECOY","amount":48},{"type":"COUNTERMEASURE_TYPE_NOISE","amount":5}]},
"actor":{"state":{"locomotion":"ACTOR_LOCOMOTION_STATE_SEATED"}}}`

func TestVPCData(t *testing.T) {
	var f map[string]any
	if err := json.Unmarshal([]byte(sampleFrame), &f); err != nil {
		t.Fatal(err)
	}
	full := map[string]any{"vehicle": map[string]any{"state": map[string]any{"lights_enabled": false,
		"flight": map[string]any{"state": map[string]any{"gear": 2.0}}, "power": map[string]any{"shield_emitters": true}}},
		"actor": map[string]any{"state": map[string]any{"uec_balance": "15001986"}},
		"weapons": map[string]any{"weapons": []any{map[string]any{"ready_to_fire": true}}}}
	known := map[string]bool{"annunciators.hit": true}
	d := vpcData(full, f, known)
	want := map[string]any{"lights_enabled": 0, "flight.state.gear": 2.0, "power.shield_emitters": 1, "actor.state.uec_balance": 15001986.0,
		"weapons.weapons.0.ready_to_fire": 1, "annunciators.hit": 0,
		"aircraftType": "GAMA_Hyun", "mech_Gear_Status": 1, "mech_Gear_Value": 100, "mcp_MasterWarning": 1,
		"mcp_AutopilotOn": 1, "plane_flare": 48, "plane_chaff": 5, "plane_RadarAltimeter": 20}
	for k, w := range want {
		if d[k] != w {
			t.Errorf("%s = %v (%T), attendu %v", k, d[k], d[k], w)
		}
	}
	if !known["lights_enabled"] {
		t.Error("clé non mémorisée")
	}
	if d := dcsKeys(nil); d["export_allowed"] != 0 {
		t.Errorf("hors du siège : %v", d)
	}
}