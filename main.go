package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

var version = "0.3.1"

func main() {
	once := flag.Bool("once", false, "affiche une trame et l'état du Game.log, puis quitte (test)")
	scDir := flag.String("sc", "", `dossier d'une branche, ex. "D:\Star Citizen\LIVE" (sinon détection automatique)`)
	port := flag.Int("port", 8742, "port de la page web")
	lan := flag.Bool("lan", false, "rend la page accessible aux autres appareils du réseau local (téléphone, tablette)")
	noBrowser := flag.Bool("nobrowser", false, "n'ouvre pas le navigateur au démarrage")
	flag.Parse()

	in := findInstall(*scDir)
	if in == nil && *scDir == "" {
		in = askInstall()
	}
	if in == nil {
		fmt.Println("Star Citizen introuvable. Indique le dossier de la branche avec -sc.")
		pause()
		os.Exit(1)
	}
	fmt.Printf("SC Telemetry Local %s · branche %s · port du jeu %d\n", version, in.Branch, in.Port)
	set, err := extractDescriptors(in.Exe)
	if err != nil {
		fmt.Println("Erreur :", err)
		pause()
		os.Exit(1)
	}
	tel, err := newTelemetry(set)
	if err != nil {
		fmt.Println("Erreur :", err)
		pause()
		os.Exit(1)
	}
	gi := newGameInfo()
	go followGameLog(in.GameLog, gi, nil)
	go tel.run(func() int { return readPort(in.Dir) })

	if *once {
		deadline := time.Now().Add(6 * time.Second)
		for time.Now().Before(deadline) && tel.JSON() == nil {
			time.Sleep(200 * time.Millisecond)
		}
		time.Sleep(1500 * time.Millisecond)
		if b := tel.JSON(); b != nil {
			fmt.Println(string(b))
		} else {
			fmt.Println("Pas de flux :", tel.LastErr)
		}
		gi.mu.Lock()
		b, _ := json.MarshalIndent(gi, "", " ")
		gi.mu.Unlock()
		fmt.Println(string(b))
		return
	}

	host := "127.0.0.1"
	if *lan {
		host = "0.0.0.0"
	}
	a := &app{in: in, tel: tel, gi: gi}
	a.port, a.lanOn = *port, *lan || loadCfg().LAN
	url := fmt.Sprintf("http://localhost:%d", *port)
	fmt.Println("Tableau de bord :", url)
	fmt.Println("Ferme cette fenêtre pour arrêter.")
	if !*noBrowser {
		go func() { time.Sleep(500 * time.Millisecond); openBrowser(url) }()
	}
	if err := a.serve(fmt.Sprintf("%s:%d", host, *port)); err != nil {
		fmt.Println("Erreur du serveur web :", err)
		pause()
		os.Exit(1)
	}
}

func pause() {
	fmt.Print("Appuie sur Entrée pour fermer…")
	fmt.Scanln()
}




