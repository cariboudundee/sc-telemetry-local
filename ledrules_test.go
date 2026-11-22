package main

import "testing"

// Les règles par défaut doivent redonner le comportement validé en jeu.
func TestDefaultRules(t *testing.T) {
	v := ledValues{nums: map[string]any{"seated": 1, "flight.state.gear": 2.0, "annunciators.hit": 1.0, "annunciators.misl": 1.0,
		"annunciators.qdrv": 1.0, "master_mode.mode": 1.0, "power.shield_generators": 1, "power.shield_emitters": 0,
		"power.powerplants": 1, "flight.settings.ifcs_coupled.enabled": 0},
		names: map[string]string{"flight.state.gear": "VEHICLE_COMPONENT_DEPLOYMENT_STATE_DEPLOYED", "annunciators.hit": "ANNUNCIATOR_STATE_NOMINAL",
			"annunciators.misl": "ANNUNCIATOR_STATE_NOMINAL", "annunciators.qdrv": "ANNUNCIATOR_STATE_NOMINAL", "master_mode.mode": "VEHICLE_MASTER_MODE_NAVIGATION"}}
	want := map[string]ledCfg{
		"Gladiator/10": {10, ledOn, rgb{0, 0, 7}}, // NAV : bleu
		"Gladiator/0":  {0, ledOn, rgb{0, 0, 7}},  // découplé
		"S-TECS/1":     {1, ledOn, rgb{7, 3, 0}},  // boucliers partiels : orange
		"S-TECS/2":     {2, ledOn, rgb{7, 0, 0}},  // train sorti : rouge
		"S-TECS/3":     {3, ledOn, rgb{0, 0, 7}},  // quantum
		"S-TECS/0":     {0, ledOn, rgb{0, 0, 7}},  // centrale
	}
	for _, l := range defaultLeds() {
		got, _ := l.eval(v)
		k := l.Device + "/" + string(rune('0'+l.LED%10))
		if l.LED == 10 {
			k = l.Device + "/10"
		}
		if got != want[k] {
			t.Errorf("%s : %+v, attendu %+v", k, got, want[k])
		}
	}
	v.names["annunciators.misl"] = "ANNUNCIATOR_STATE_MAJOR"
	if got, _ := defaultLeds()[0].eval(v); got.Mode != ledFast || got.Color != (rgb{7, 0, 0}) {
		t.Errorf("missile : %+v", got)
	}
	v.nums["seated"] = 0
	for _, l := range defaultLeds() {
		if got, _ := l.eval(v); got.Mode != ledOff {
			t.Errorf("hors du siège, LED %d allumée", l.LED)
		}
	}
	if (ledCond{"power.powerplants", ">=", "1"}).match(v) != true || (ledCond{"absent", "==", "0"}).match(v) {
		t.Error("comparaisons")
	}
}