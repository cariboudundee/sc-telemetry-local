package main

// Démarrage automatique : une entrée « Exécuter » de Windows (compte de l'utilisateur) lance l'outil
// à l'ouverture de session en mode attente, sans fenêtre. Dès que RSI Launcher ou Star Citizen tourne,
// la fenêtre de l'outil apparaît et tout démarre ; quand les deux sont fermés depuis 1 minute, l'outil
// se remet en attente.

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const runName = "SC Telemetry Local"

var gameProcs = []string{"RSI Launcher.exe", "StarCitizen.exe"}

var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procAllocConsole = kernel32.NewProc("AllocConsole")
	procSetTitle     = kernel32.NewProc("SetConsoleTitleW")
)

func exePath() string { p, _ := os.Executable(); return p }

// alog : petit journal du mode attente (%APPDATA%\SCTelemetryLocal\autostart.log), utile sans fenêtre.
func alog(format string, a ...any) {
	p := filepath.Join(filepath.Dir(cfgPath()), "autostart.log")
	os.MkdirAll(filepath.Dir(p), 0o755)
	if st, err := os.Stat(p); err == nil && st.Size() > 64<<10 {
		os.Remove(p)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, a...))
}

func autostartOn() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runName)
	return err == nil
}

func setAutostart(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(runName); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	return k.SetStringValue(runName, fmt.Sprintf(`"%s" -wait`, exePath()))
}

// refreshAutostart : si l'exe a été déplacé ou remplacé par une nouvelle version, l'entrée suit.
func refreshAutostart() {
	if autostartOn() {
		setAutostart(true)
	}
}

// relaunchDetached : relance l'outil sans fenêtre (mode attente) et quitte celui-ci.
func relaunchDetached(extra ...string) {
	args := append([]string{"-wait", "-detached"}, extra...)
	cmd := exec.Command(exePath(), args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
	cmd.Dir = filepath.Dir(exePath())
	if err := cmd.Start(); err != nil {
		alog("relance en attente impossible : %v", err)
		fmt.Println("Mise en attente impossible :", err)
		return
	}
	alog("relancé en attente (pid %d)", cmd.Process.Pid)
	os.Exit(0)
}

// waitForGame : sans fenêtre, attend RSI Launcher ou Star Citizen, puis ouvre une fenêtre.
func waitForGame() (woken bool) {
	alog("en attente de RSI Launcher / Star Citizen (pid %d)", os.Getpid())
	for !running(gameProcs...) {
		if ev, _ := windows.WaitForSingleObject(wakeEvent, 5000); ev == windows.WAIT_OBJECT_0 {
			woken = true
			break
		}
	}
	alog("jeu ou launcher détecté")
	if r, _, e := procAllocConsole.Call(); r == 0 {
		alog("AllocConsole : %v", e)
	}
	if f, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
		os.Stdout, os.Stderr = f, f
	}
	if f, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil {
		os.Stdin = f
	}
	t, _ := windows.UTF16PtrFromString("SC Telemetry Local")
	procSetTitle.Call(uintptr(unsafe.Pointer(t)))
	if woken {
		fmt.Println("Lancement manuel : démarrage.")
	} else {
		fmt.Println("RSI Launcher / Star Citizen détecté : démarrage.")
	}
	return woken
}

// backToWait : une fois le jeu et le launcher fermés depuis 1 minute, retour en attente
// (après un réveil manuel, seulement si le jeu ou le launcher a tourné entre-temps).
func backToWait() {
	idle, seen := time.Time{}, false
	for range time.Tick(10 * time.Second) {
		if running(gameProcs...) {
			idle, seen = time.Time{}, true
			continue
		}
		if !seen {
			continue
		}
		if idle.IsZero() {
			idle = time.Now()
		} else if time.Since(idle) > time.Minute {
			alog("jeu et launcher fermés : retour en attente")
			relaunchDetached()
		}
	}
}

// Une seule instance à la fois. Un nouveau lancement pendant qu'une instance attend la réveille
// (fenêtre + page) ; si une instance est déjà active, il ouvre simplement la page.
var wakeEvent windows.Handle

func singleInstance() bool {
	name, _ := windows.UTF16PtrFromString(`Local\SCTelemetryLocal`)
	ev, _ := windows.UTF16PtrFromString(`Local\SCTelemetryLocal-wake`)
	wakeEvent, _ = windows.CreateEvent(nil, 0, 0, ev)
	for i := 0; i < 20; i++ { // après une relance, l'instance précédente libère la place
		h, err := windows.CreateMutex(nil, false, name)
		if err == nil {
			return true // gardé jusqu'à la fin du processus
		}
		if h != 0 {
			windows.CloseHandle(h)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func wakeOther() {
	if wakeEvent != 0 {
		windows.SetEvent(wakeEvent)
	}
}

func autostartHandler(mux *http.ServeMux) {
	mux.HandleFunc("/autostart", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if !localOnly(w, r) {
				return
			}
			if err := setAutostart(r.URL.Query().Get("on") == "1"); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"on":%v}`, autostartOn())
	})
}