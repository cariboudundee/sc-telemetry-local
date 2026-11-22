package main

// Changement de branche (LIVE / PTU / EPTU…) en cours de route : la télémétrie se décode avec le
// schéma lu dans l'exécutable de la branche lancée. Si une autre branche démarre, l'outil se relance
// sur celle-ci (même fenêtre, mêmes options).

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// activeBranch : dossier de la branche voisine dont le Game.log est le plus récent, si le jeu tourne.
func activeBranch(cur *Install) string {
	if !running("StarCitizen.exe") {
		return ""
	}
	best, bestT := "", time.Time{}
	entries, _ := os.ReadDir(filepath.Dir(cur.Dir))
	for _, e := range entries {
		d := filepath.Join(filepath.Dir(cur.Dir), e.Name())
		if !e.IsDir() || !isBranch(d) {
			continue
		}
		if st, err := os.Stat(filepath.Join(d, "Game.log")); err == nil && st.ModTime().After(bestT) {
			best, bestT = d, st.ModTime()
		}
	}
	return best
}

func watchBranch(cur *Install) {
	for range time.Tick(5 * time.Second) {
		d := activeBranch(cur)
		if d == "" || strings.EqualFold(d, cur.Dir) {
			continue
		}
		fmt.Printf("Star Citizen tourne maintenant depuis %s : redémarrage de l'outil sur cette branche…\n", filepath.Base(d))
		args := []string{}
		for _, a := range os.Args[1:] {
			if !strings.HasPrefix(a, "-sc") && !strings.HasPrefix(a, "--sc") {
				args = append(args, a)
			}
		}
		args = append(args, "-nobrowser", "-sc", d)
		cmd := exec.Command(os.Args[0], args...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Println("Redémarrage impossible :", err)
			continue
		}
		os.Exit(0)
	}
}