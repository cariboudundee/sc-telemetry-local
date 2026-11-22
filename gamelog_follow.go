package main

import (

	"io"
	"os"
	"strings"
	"time"
)




func (g *GameInfo) line(l string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if m := reNotif.FindStringSubmatch(l); m != nil {
		g.notification(m[1], strings.ToLower(m[2]))
	} else if m := reJoin.FindStringSubmatch(l); m != nil {
		g.Server, g.Shard = m[1]+":"+m[2], m[3]
	} else if m := reShard.FindStringSubmatch(l); m != nil {
		g.Shard = m[1]
	} else if m := reHandle.FindStringSubmatch(l); m != nil {
		g.Handle = m[1]
	} else if m := reHandle2.FindStringSubmatch(l); m != nil && g.Handle == "" {
		g.Handle = m[1]
	} else if m := reBranch.FindStringSubmatch(l); m != nil {
		g.Branch = m[1]
	} else if m := reCL.FindStringSubmatch(l); m != nil {
		g.Build = m[1]
	} else if strings.Contains(l, "<EndMission>") {
		if m := reEndType.FindStringSubmatch(l); m != nil {
			g.LastResult = "Fin de contrat : " + m[1]
		}
		if m := reEndId.FindStringSubmatch(l); m != nil {
			g.end(strings.ToLower(m[1]))
		}
	}
}

// followGameLog lit le journal depuis le début puis suit les ajouts ; repart à zéro si le fichier rétrécit (nouvelle session).
func followGameLog(path string, g *GameInfo, onReset func()) {
	var off int64
	pending := ""
	for {
		st, err := os.Stat(path)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if st.Size() < off {
			off, pending = 0, ""
			g.mu.Lock()
			*g = GameInfo{Missions: map[string]*Mission{}, LastNotif: []string{}}
			g.mu.Unlock()
			if onReset != nil {
				onReset()
			}
		}
		if st.Size() > off {
			if f, err := os.Open(path); err == nil {
				f.Seek(off, 0)
				buf := make([]byte, st.Size()-off)
				n, _ := io.ReadFull(f, buf)
				f.Close()
				off += int64(n)
				lines := strings.Split(pending+string(buf[:n]), "\n")
				pending = lines[len(lines)-1]
				var joined string
				for _, l := range lines[:len(lines)-1] {
					l = strings.TrimRight(l, "\r")
					// une notification peut tenir sur plusieurs lignes
					if joined != "" {
						l = joined + " " + l
						joined = ""
					}
					if strings.Contains(l, `Added notification "`) && !strings.Contains(l, "to queue") {
						joined = l
						continue
					}
					g.line(l)
				}
			}
		}
		time.Sleep(time.Second)
	}
}

