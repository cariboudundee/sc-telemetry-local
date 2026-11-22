package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Reproduit le lancement du jeu : Game.log rétrécit (nouvelle session) pendant que l'outil le suit.
func TestGameLogReset(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Game.log")
	os.WriteFile(p, []byte("<2026-11-22T10:00:00.000Z> <Join PU> address[1.2.3.4] port[64300] shard[pub_test_1]\n"), 0o644)
	g := newGameInfo()
	go followGameLog(p, g, nil)
	time.Sleep(1500 * time.Millisecond)
	os.WriteFile(p, []byte("x\n"), 0o644) // le jeu recrée un journal plus court
	time.Sleep(2500 * time.Millisecond)
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Shard != "" {
		t.Fatalf("infos non remises à zéro : %q", g.Shard)
	}
}
