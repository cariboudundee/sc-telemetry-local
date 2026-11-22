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
	d := vpcData(f)
	want := map[string]any{"aircraftType": "GAMA_Hyun", "mech_Gear_Status": 1, "mech_Gear_Value": 100, "mcp_MasterWarning": 1,
		"sc_hit": 1, "sc_missile": 1, "sc_shields": 1, "sc_decoupled": 1, "sc_nav": 1, "mcp_AutopilotOn": 1,
		"plane_flare": 48, "plane_chaff": 5, "plane_RadarAltimeter": 20, "sc_hydrogen": 95, "sc_lights": 1}
	for k, w := range want {
		if d[k] != w {
			t.Errorf("%s = %v, attendu %v", k, d[k], w)
		}
	}
	if d := vpcData(nil); d["sc_seated"] != 0 {
		t.Errorf("hors du siège : %v", d)
	}
}