package main

// Assistant de mise en route : vérifie chaque prérequis et active la variable d'environnement sur demande.

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const envName = "GRPC_CLIENT_SERVER_ENABLED"

type setupState struct {
	Found     bool     `json:"found"`
	Branch    string   `json:"branch"`
	Dir       string   `json:"dir"`
	Others    []string `json:"others"`
	Env       bool     `json:"env"`
	Launcher  bool     `json:"launcher"`
	Game      bool     `json:"game"`
	Port      int      `json:"port"`
	Listening bool     `json:"listening"`
	Connected bool     `json:"connected"`
	EnvSetNow bool     `json:"envSetNow"` // activée par l'assistant pendant cette session
}

var envSetAt time.Time

func envEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(envName)
	return err == nil && strings.EqualFold(strings.TrimSpace(v), "true")
}

// enableEnv : équivalent de « setx GRPC_CLIENT_SERVER_ENABLED true » (variable de l'utilisateur, pas du système).
func enableEnv() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetStringValue(envName, "true"); err != nil {
		return err
	}
	envSetAt = time.Now()
	// prévient Windows (et l'Explorateur) que l'environnement a changé
	env, _ := syscall.UTF16PtrFromString("Environment")
	var res uintptr
	windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW").Call(
		0xFFFF, 0x001A, 0, uintptr(unsafe.Pointer(env)), 0x0002, 5000, uintptr(unsafe.Pointer(&res)))
	return nil
}

func running(names ...string) bool {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		exe := strings.ToLower(windows.UTF16ToString(e.ExeFile[:]))
		for _, n := range names {
			if exe == strings.ToLower(n) {
				return true
			}
		}
	}
	return false
}

func listening(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func otherBranches(cur *Install) []string {
	var out []string
	for _, drv := range "CDEFGHIJ" {
		for _, b := range bases {
			for _, br := range branches {
				dir := filepath.Join(string(drv)+`:\`, b, br)
				if cur != nil && strings.EqualFold(dir, cur.Dir) {
					continue
				}
				if _, err := os.Stat(filepath.Join(dir, "Bin64", "StarCitizen.exe")); err == nil {
					out = append(out, br)
				}
			}
		}
	}
	return out
}

func (a *app) setupState() setupState {
	s := setupState{Env: envEnabled(), Launcher: running("RSI Launcher.exe"), Game: running("StarCitizen.exe")}
	if a.in != nil {
		s.Found, s.Branch, s.Dir = true, a.in.Branch, a.in.Dir
		s.Port = readPort(a.in.Dir)
		s.Listening = listening(s.Port)
	}
	s.Others = otherBranches(a.in)
	s.EnvSetNow = !envSetAt.IsZero()
	a.tel.mu.RLock()
	s.Connected = a.tel.Connected
	a.tel.mu.RUnlock()
	return s
}

// localOnly : les actions qui modifient le PC ne sont acceptées que depuis ce PC, et seulement depuis notre page
// (l'en-tête X-SCTL oblige un autre site à une requête préalable que nous ne validons pas).
func localOnly(w http.ResponseWriter, r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	if r.Method != http.MethodPost || ip == nil || !ip.IsLoopback() || r.Header.Get("X-SCTL") != "1" {
		http.Error(w, "refusé", http.StatusForbidden)
		return false
	}
	return true
}
