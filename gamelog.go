package main

// Lecture du Game.log : pseudo, shard, serveur, version, juridiction, contrats et objectifs.

import (
	"regexp"
	"strings"
	"sync"
)

type Mission struct {
	Name      string `json:"name,omitempty"`
	Objective string `json:"objective,omitempty"`
}

type GameInfo struct {
	mu           sync.Mutex
	Handle       string              `json:"handle,omitempty"`
	Shard        string              `json:"shard,omitempty"`
	Server       string              `json:"server,omitempty"`
	Branch       string              `json:"branch,omitempty"`
	Build        string              `json:"build,omitempty"`
	Jurisdiction string              `json:"jurisdiction,omitempty"`
	Monitored    *bool               `json:"monitored,omitempty"`
	Current      string              `json:"current,omitempty"`
	Missions     map[string]*Mission `json:"missions"`
	LastResult   string              `json:"lastResult,omitempty"`
	LastNotif    []string            `json:"lastNotifications"`
}

const zeroID = "00000000-0000-0000-0000-000000000000"

var (
	reNotif    = regexp.MustCompile(`Added notification "([\s\S]*?)" \[\d+\] to queue[\s\S]*?MissionId: \[([0-9a-fA-F-]+)\]`)
	reJoin     = regexp.MustCompile(`<Join PU> address\[([^\]]+)\] port\[(\d+)\] shard\[([^\]]+)\]`)
	reShard    = regexp.MustCompile(`<Update Shard Id> New Shard Id:\s*([A-Za-z0-9_]+)`)
	reHandle   = regexp.MustCompile(`<Legacy login response>.*?Handle\[([^\]]+)\]`)
	reHandle2  = regexp.MustCompile(`AccountLoginCharacterStatus_Character.*?-\s*name\s+(\S+)\s*-\s*state`)
	reBranch   = regexp.MustCompile(`Branch:\s*(sc-[\w.-]+)`)
	reCL       = regexp.MustCompile(`Changelist:\s*(\d+)`)
	reEndId    = regexp.MustCompile(`MissionId\[([0-9a-fA-F-]+)\]`)
	reEndType  = regexp.MustCompile(`CompletionType\[([^\]]+)\]`)
	reTime     = regexp.MustCompile(`<\d{4}-\d\d-\d\dT[^>]*>|\[(Notice|Error|Warning)\]|</?[A-Za-z][A-Za-z0-9]*>`) // horodatage, niveau, balises du jeu (<EM4>…)
	reSpace    = regexp.MustCompile(`\s+`)
	reJuris    = regexp.MustCompile(`(?i)^Entered (.+?) Jurisdiction$`)
	reEndHead  = regexp.MustCompile(`(?i)^Contract (Complete|Completed|Failed|Abandoned|Withdrawn|Expired)$`)
)

func newGameInfo() *GameInfo { return &GameInfo{Missions: map[string]*Mission{}, LastNotif: []string{}} }

func clean(s string) string {
	s = reSpace.ReplaceAllString(reTime.ReplaceAllString(s, " "), " ")
	return strings.TrimRight(strings.TrimSpace(s), ": ")
}

func (g *GameInfo) mission(id string) *Mission {
	if g.Missions[id] == nil {
		g.Missions[id] = &Mission{}
	}
	return g.Missions[id]
}

func (g *GameInfo) end(id string) {
	delete(g.Missions, id)
	if g.Current == id {
		g.Current = ""
		for k := range g.Missions {
			g.Current = k
		}
	}
}

func (g *GameInfo) notification(text, id string) {
	t := clean(text)
	if t == "" {
		return
	}
	g.LastNotif = append([]string{t}, g.LastNotif...)
	if len(g.LastNotif) > 5 {
		g.LastNotif = g.LastNotif[:5]
	}
	head, rest := t, ""
	if i := strings.Index(t, ": "); i >= 0 {
		head, rest = t[:i], clean(t[i+2:])
	}
	hasID := id != "" && id != zeroID
	switch {
	case reJuris.MatchString(head):
		g.Jurisdiction = reJuris.FindStringSubmatch(head)[1]
	case strings.HasPrefix(strings.ToLower(head), "entered monitored space"):
		v := true
		g.Monitored = &v
	case strings.Contains(strings.ToLower(head), "monitored space") && (strings.HasPrefix(strings.ToLower(head), "left") || strings.HasPrefix(strings.ToLower(head), "leaving") || strings.HasPrefix(strings.ToLower(head), "exited")):
		v := false
		g.Monitored = &v
	case strings.EqualFold(head, "Contract Accepted") && hasID:
		if rest != "" {
			g.mission(id).Name = rest
		}
		g.Current = id
	case strings.EqualFold(head, "New Objective") && hasID:
		g.mission(id).Objective = rest
		g.Current = id
	case strings.EqualFold(head, "Objective Complete") && hasID:
		if m := g.mission(id); m.Objective == "" || m.Objective == rest {
			m.Objective = "✓ " + rest
		}
	case reEndHead.MatchString(head) && hasID:
		g.LastResult = head
		if rest != "" {
			g.LastResult += " : " + rest
		}
		g.end(id)
	}
}


