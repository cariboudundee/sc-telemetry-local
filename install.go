package main

// Recherche de l'installation de Star Citizen et de la branche active (LIVE, PTU…).
// Sources, de la plus sûre à la plus large : dossier choisi par l'utilisateur, jeu en cours,
// journal du RSI Launcher, dossier de l'exe et ses parents, emplacements courants, recherche sur les disques.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Install struct {
	Branch  string
	Dir     string
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

// isBranch : dir est-il un dossier de branche (contient Bin64\StarCitizen.exe) ?
func isBranch(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Bin64", "StarCitizen.exe"))
	return err == nil
}

// expand : à partir d'un dossier quelconque, trouve les dossiers de branche qu'il désigne ou contient.
func expand(dir string, add func(string)) {
	if dir == "" {
		return
	}
	dir = strings.Trim(strings.TrimSpace(dir), `"`)
	if strings.EqualFold(filepath.Base(dir), "Bin64") {
		dir = filepath.Dir(dir)
	}
	if isBranch(dir) {
		add(dir)
		return
	}
	for _, sub := range []string{"", "StarCitizen", `Roberts Space Industries\StarCitizen`} {
		for _, br := range branches {
			if p := filepath.Join(dir, sub, br); isBranch(p) {
				add(p)
			}
		}
	}
}

func candidates(override string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		k := strings.ToLower(filepath.Clean(p))
		if !seen[k] {
			seen[k] = true
			out = append(out, filepath.Clean(p))
		}
	}
	if override != "" {
		expand(override, add)
		return out
	}
	expand(loadCfg().SCDir, add)
	expand(runningGameDir(), add)
	for _, p := range launcherDirs() {
		expand(p, add)
	}
	if exe, err := os.Executable(); err == nil {
		for p, i := filepath.Dir(exe), 0; i < 4; p, i = filepath.Dir(p), i+1 {
			expand(p, add)
		}
	}
	for _, drv := range drives() {
		for _, b := range bases {
			expand(filepath.Join(drv, b), add)
		}
	}
	if len(out) == 0 { // dernier recours : recherche limitée sur les disques
		for _, drv := range drives() {
			scan(drv, 0, add)
		}
	}
	return out
}

func drives() []string {
	var out []string
	for c := 'C'; c <= 'Z'; c++ {
		d := string(c) + `:\`
		if t := windows.GetDriveType(windows.StringToUTF16Ptr(d)); t == windows.DRIVE_FIXED {
			out = append(out, d)
		}
	}
	return out
}

var skipDirs = map[string]bool{"windows": true, "$recycle.bin": true, "system volume information": true, "programdata": true, "users": true, "appdata": true, "node_modules": true}

// scan : parcourt les dossiers jusqu'à 4 niveaux pour trouver un dossier de branche.
func scan(dir string, depth int, add func(string)) {
	if depth > 4 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || skipDirs[strings.ToLower(e.Name())] || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		for _, br := range branches {
			if strings.EqualFold(e.Name(), br) && isBranch(p) {
				add(p)
			}
		}
		scan(p, depth+1, add)
	}
}

var launchRe = regexp.MustCompile(`Launching Star Citizen \S+ from \((.+?)\)`)

// launcherDirs : dossiers de lancement notés dans le journal du RSI Launcher.
func launcherDirs() []string {
	var out []string
	for _, name := range []string{"log.log", "log.old.log"} {
		f, err := os.Open(filepath.Join(os.Getenv("APPDATA"), "rsilauncher", "logs", name))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			if m := launchRe.FindStringSubmatch(sc.Text()); m != nil {
				out = append(out, strings.ReplaceAll(m[1], `\\`, `\`))
			}
		}
		f.Close()
	}
	return out
}

// runningGameDir : dossier de branche du StarCitizen.exe en cours d'exécution, si accessible.
func runningGameDir() string {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if !strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), "StarCitizen.exe") {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, e.ProcessID)
		if err != nil {
			return ""
		}
		buf := make([]uint16, windows.MAX_LONG_PATH)
		n := uint32(len(buf))
		err = windows.QueryFullProcessImageName(h, 0, &buf[0], &n)
		windows.CloseHandle(h)
		if err == nil {
			return filepath.Dir(filepath.Dir(windows.UTF16ToString(buf[:n])))
		}
	}
	return ""
}

// findInstall choisit la branche dont le Game.log est le plus récent (= celle qu'on joue).
func findInstall(override string) *Install {
	var best *Install
	var bestT time.Time
	for _, dir := range candidates(override) {
		in := &Install{Branch: strings.ToUpper(filepath.Base(dir)), Dir: dir, Exe: filepath.Join(dir, "Bin64", "StarCitizen.exe"), GameLog: filepath.Join(dir, "Game.log")}
		t := time.Time{}
		if st, err := os.Stat(in.GameLog); err == nil {
			t = st.ModTime()
		}
		if best == nil || t.After(bestT) {
			best, bestT = in, t
		}
	}
	if best != nil {
		best.Port = readPort(best.Dir)
	}
	return best
}

// askInstall : dernier recours, demande le dossier dans la console et le mémorise.
func askInstall() *Install {
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("Star Citizen introuvable automatiquement.")
		fmt.Println(`Colle (clic droit) ou glisse ici le dossier du jeu, par exemple  D:\Jeux\StarCitizen\LIVE , puis Entrée.`)
		fmt.Print("> ")
		line, err := in.ReadString('\n')
		if err != nil {
			return nil
		}
		line = strings.Trim(strings.TrimSpace(line), `"`)
		if line == "" {
			return nil
		}
		if found := findInstall(line); found != nil {
			c := loadCfg()
			c.SCDir = line
			saveCfg(c)
			return found
		}
		fmt.Println("Aucun StarCitizen.exe trouvé dans ce dossier (attendu : …\\LIVE\\Bin64\\StarCitizen.exe). Réessaie.")
	}
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
