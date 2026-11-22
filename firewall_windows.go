package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// virtualIf : cartes réseau virtuelles (Hyper-V, WSL, VPN…) qu'un téléphone ne peut pas joindre.
func virtualIf(name string) bool {
	n := strings.ToLower(name)
	for _, k := range []string{"vethernet", "wsl", "hyper-v", "virtualbox", "vmware", "vpn", "tailscale", "zerotier", "loopback", "bluetooth"} {
		if strings.Contains(n, k) {
			return true
		}
	}
	return false
}

// SHELLEXECUTEINFOW (shell32), absente de golang.org/x/sys/windows
type shellExecuteInfo struct {
	size        uint32
	mask        uint32
	hwnd        uintptr
	verb        *uint16
	file        *uint16
	parameters  *uint16
	directory   *uint16
	show        int32
	instApp     uintptr
	idList      uintptr
	class       *uint16
	keyClass    uintptr
	hotKey      uint32
	iconMonitor uintptr
	process     windows.Handle
}

var procShellExecuteEx = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// addFirewallRule : demande l'élévation (UAC) à Windows et crée une règle d'autorisation pour cet .exe,
// sur le port de la page, réseaux privés seulement ; supprime les règles de refus créées automatiquement pour lui.
func addFirewallRule(port int) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe = strings.ReplaceAll(exe, "'", "''")
	ps := fmt.Sprintf(`$p='%s'; Get-NetFirewallApplicationFilter -Program $p -ErrorAction SilentlyContinue | Get-NetFirewallRule | Where-Object { $_.Action -eq 'Block' -or $_.DisplayName -eq 'SC Telemetry Local' } | Remove-NetFirewallRule -ErrorAction SilentlyContinue; New-NetFirewallRule -DisplayName 'SC Telemetry Local' -Direction Inbound -Program $p -Protocol TCP -LocalPort %d -Action Allow -Profile Private | Out-Null`, exe, port)
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString("powershell.exe")
	args, _ := syscall.UTF16PtrFromString("-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -Command \"" + strings.ReplaceAll(ps, `"`, `\"`) + "\"")
	info := &shellExecuteInfo{verb: verb, file: file, parameters: args, show: windows.SW_HIDE, mask: 0x00000040}
	info.size = uint32(unsafe.Sizeof(*info))
	if ok, _, err := procShellExecuteEx.Call(uintptr(unsafe.Pointer(info))); ok == 0 {
		return err // refusé ou annulé par l'utilisateur
	}
	if info.process != 0 {
		windows.WaitForSingleObject(info.process, 30000)
		var code uint32
		windows.GetExitCodeProcess(info.process, &code)
		windows.CloseHandle(info.process)
		if code != 0 {
			return fmt.Errorf("code de sortie %d", code)
		}
	}
	return nil
}

func (a *app) firewallHandler(mux *http.ServeMux) {
	mux.HandleFunc("/firewall", func(w http.ResponseWriter, r *http.Request) {
		if !localOnly(w, r) {
			return
		}
		if err := addFirewallRule(a.port); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		a.stopLAN() // rouvre les ports pour que la nouvelle règle s'applique
		a.startLAN()
		w.Write([]byte("ok"))
	})
}


