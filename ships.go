package main

// Liste des vaisseaux et véhicules du jeu, pour choisir le vaisseau d'un tableau : noms des fichiers
// d'implémentation des véhicules dans Data.p4k (lecture du seul répertoire de l'archive, ~1 s, en cache).

import (
	"archive/zip"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	shipsOnce sync.Once
	shipsList []string
)

var shipSkip = []string{"low_poly", "ea_destructable", "probe_", "mite_simpod", "_dead"}

func gameShips(branchDir string) []string {
	shipsOnce.Do(func() {
		r, err := zip.OpenReader(filepath.Join(branchDir, "Data.p4k"))
		if err != nil {
			return
		}
		defer r.Close()
		seen := map[string]bool{}
		for _, f := range r.File {
			l := strings.ToLower(f.Name)
			if !strings.Contains(l, `implementations\xml\`) || !strings.HasSuffix(l, ".xml") {
				continue
			}
			name := strings.TrimSuffix(f.Name[strings.LastIndexAny(f.Name, `\/`)+1:], filepath.Ext(f.Name))
			key := strings.ToLower(name)
			skip := seen[key]
			for _, s := range shipSkip {
				skip = skip || strings.Contains(key, s)
			}
			if !skip {
				seen[key] = true
				shipsList = append(shipsList, name)
			}
		}
		sort.Slice(shipsList, func(i, j int) bool { return strings.ToLower(shipsList[i]) < strings.ToLower(shipsList[j]) })
	})
	return shipsList
}