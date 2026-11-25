package main

// Tableaux de bord personnalisés (page custom.html) : enregistrés dans
// %APPDATA%\SCTelemetryLocal\dashboards.json. La page garde son propre modèle par défaut ;
// le serveur ne fait que lire, valider grossièrement et écrire le fichier.

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// schemaFields : tous les champs possibles de la trame (lus dans le schéma du jeu), sous leur nom
// d'origine à points, pour proposer la liste même jeu fermé. Listes : premier élément (.0).
func schemaFields(md protoreflect.MessageDescriptor) []string {
	var out []string
	var walk func(m protoreflect.MessageDescriptor, prefix string, depth int)
	walk = func(m protoreflect.MessageDescriptor, prefix string, depth int) {
		if depth > 8 {
			return
		}
		fs := m.Fields()
		for i := 0; i < fs.Len(); i++ {
			f := fs.Get(i)
			if f.IsMap() {
				continue
			}
			name := prefix + string(f.Name())
			if f.IsList() {
				name += ".0"
			}
			if f.Kind() == protoreflect.MessageKind || f.Kind() == protoreflect.GroupKind {
				walk(f.Message(), name+".", depth+1)
				continue
			}
			out = append(out, strings.TrimPrefix(name, "vehicle.state."))
		}
	}
	walk(md, "", 0)
	sort.Strings(out)
	return out
}

const maxDash = 2 << 20

func dashPath() string { return filepath.Join(filepath.Dir(cfgPath()), "dashboards.json") }

func (a *app) dashHandler(mux *http.ServeMux) {
	mux.HandleFunc("/dash", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if !localOnly(w, r) {
				return
			}
			b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxDash))
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			var probe struct {
				Boards []json.RawMessage `json:"boards"`
			}
			if err := json.Unmarshal(b, &probe); err != nil || probe.Boards == nil {
				http.Error(w, "format invalide", 400)
				return
			}
			os.MkdirAll(filepath.Dir(dashPath()), 0o755)
			tmp := dashPath() + ".tmp"
			if err := os.WriteFile(tmp, b, 0o644); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if err := os.Rename(tmp, dashPath()); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if b, err := os.ReadFile(dashPath()); err == nil && json.Valid(b) {
			w.Write(b)
			return
		}
		w.Write([]byte("null"))
	})
	mux.HandleFunc("/ships", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gameShips(a.in.Dir))
	})
	// Tous les champs de la télémétrie (noms d'origine), pour les listes de la page.
	mux.HandleFunc("/fields", func(w http.ResponseWriter, r *http.Request) {
		v := a.ledValues()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"values": v.nums, "names": v.names, "schema": schemaFields(a.tel.out)})
	})
}