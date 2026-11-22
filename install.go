package main

// Recherche de l'installation de Star Citizen et de la branche active (LIVE, PTU…).

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

type Install struct {
	Branch  string // LIVE, PTU…
	Dir     string // ...\StarCitizen\LIVE
	Exe     string
	GameLog string
	Port    int
}

var bases = []string{
	`Program Files\Roberts Space Industries\StarCitizen`,
	`Roberts Space Industries\StarCitizen`,
	`Games\Roberts Space Industries\StarCitizen`,
	`StarCitizen`, `Games\StarCitizen`,
}
var branches = []string{"LIVE", "PTU", "EPTU", "HOTFIX", "TECH-PREVIEW"}

// findInstall choisit la branche dont le Game.log est le plus récent (= celle qu'on joue).
func findInstall(override string) *Install {
	var best *Install
	var bestT time.Time
	try := func(dir, br string) {
		exe := filepath.Join(dir, "Bin64", "StarCitizen.exe")
		if _, err := os.Stat(exe); err != nil {
			return
		}
		in := &Install{Branch: br, Dir: dir, Exe: exe, GameLog: filepath.Join(dir, "Game.log")}
		t := time.Time{}
		if st, err := os.Stat(in.GameLog); err == nil {
			t = st.ModTime()
		}
		if best == nil || t.After(bestT) {
			best, bestT = in, t
		}
	}
	if override != "" {
		try(override, filepath.Base(override))
	} else {
		for _, drv := range "CDEFGHIJ" {
			for _, b := range bases {
				for _, br := range branches {
					try(filepath.Join(string(drv)+`:\`, b, br), br)
				}
			}
		}
	}
	if best != nil {
		best.Port = readPort(best.Dir)
	}
	return best
}

var portRe = regexp.MustCompile(`(?mi)^\s*grpc\.client\.server\.port\s*=\s*(\d+)`)

// readPort : port réglé dans user.cfg, sinon 2942 (valeur de la variable GRPC_CLIENT_SERVER_ENABLED)
func readPort(dir string) int {
	b, err := os.ReadFile(filepath.Join(dir, "user.cfg"))
	if err == nil {
		if m := portRe.FindSubmatch(b); m != nil {
			if p, err := strconv.Atoi(string(m[1])); err == nil {
				return p
			}
		}
	}
	return 2942
}
