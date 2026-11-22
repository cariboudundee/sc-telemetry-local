package main

import (
	"encoding/hex"
	"testing"
)

// Comparaison avec la bibliothèque pyvkb (référence vérifiée sur un Gladiator et un S-TECS).
func TestLEDBytes(t *testing.T) {
	cs := []ledCfg{{10, ledOn, rgb{7, 0, 0}}, {11, ledFast, rgb{0, 7, 0}}, {0, ledSlow, rgb{0, 0, 7}}, {1, ledOn, rgb{7, 4, 0}}}
	var b []byte
	for _, c := range cs {
		b = append(b, c.bytes()...)
	}
	if got := hex.EncodeToString(b); got != "0a0700040b38000c00c0010801270004" {
		t.Fatalf("octets LED : %s", got)
	}
	body := append([]byte{0x12, 0x34, byte(len(cs))}, b...)
	if got := hex.EncodeToString(ledChecksum(len(cs), body)); got != "3175" {
		t.Fatalf("somme de contrôle : %s", got)
	}
}