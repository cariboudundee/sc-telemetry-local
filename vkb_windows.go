package main

// LED des contrôleurs VKB (Gladiator, S-TECS…) : envoi du rapport HID « feature » 0x59,
// le même que VKBDevCfg et MobiFlight. Rien n'est enregistré dans l'appareil : il reprend
// son comportement d'origine quand on le débranche/rebranche.

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const vkbVID = 0x231D

var (
	hidDLL               = windows.NewLazySystemDLL("hid.dll")
	procHidGetAttributes = hidDLL.NewProc("HidD_GetAttributes")
	procHidGetProduct    = hidDLL.NewProc("HidD_GetProductString")
	procHidGetPreparsed  = hidDLL.NewProc("HidD_GetPreparsedData")
	procHidFreePreparsed = hidDLL.NewProc("HidD_FreePreparsedData")
	procHidPGetCaps      = hidDLL.NewProc("HidP_GetCaps")
	procHidSetFeature    = hidDLL.NewProc("HidD_SetFeature")
	guidHID              = windows.GUID{Data1: 0x4D1E55B2, Data2: 0xF16F, Data3: 0x11CF, Data4: [8]byte{0x88, 0xCB, 0x00, 0x11, 0x11, 0x00, 0x00, 0x30}}
)

type hidAttributes struct {
	Size      uint32
	VendorID  uint16
	ProductID uint16
	Version   uint16
}

type hidCaps struct {
	Usage, UsagePage                                      uint16
	InputLen, OutputLen, FeatureLen                       uint16
	Reserved                                              [17]uint16
	Rest                                                  [10]uint16
}

// vkbDevice : une interface HID d'un appareil VKB qui accepte le rapport LED.
type vkbDevice struct {
	Path, Name string
	PID        uint16
	FeatLen    int
}

// findVKB liste les appareils VKB branchés qui acceptent le rapport LED (129 octets).
func findVKB() []vkbDevice {
	paths, err := windows.CM_Get_Device_Interface_List("", &guidHID, windows.CM_GET_DEVICE_INTERFACE_LIST_PRESENT)
	if err != nil {
		return nil
	}
	var out []vkbDevice
	for _, p := range paths {
		if !strings.Contains(strings.ToLower(p), "vid_231d") {
			continue
		}
		h, err := openHID(p)
		if err != nil {
			continue
		}
		var at hidAttributes
		at.Size = uint32(unsafe.Sizeof(at))
		procHidGetAttributes.Call(uintptr(h), uintptr(unsafe.Pointer(&at)))
		var pp uintptr
		var caps hidCaps
		if r, _, _ := procHidGetPreparsed.Call(uintptr(h), uintptr(unsafe.Pointer(&pp))); r != 0 {
			procHidPGetCaps.Call(pp, uintptr(unsafe.Pointer(&caps)))
			procHidFreePreparsed.Call(pp)
		}
		name := make([]uint16, 128)
		procHidGetProduct.Call(uintptr(h), uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)*2))
		windows.CloseHandle(h)
		if at.VendorID != vkbVID || caps.FeatureLen < 64 {
			continue
		}
		out = append(out, vkbDevice{Path: p, Name: strings.TrimSpace(windows.UTF16ToString(name)), PID: at.ProductID, FeatLen: int(caps.FeatureLen)})
	}
	return out
}

func openHID(path string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
}

// Modes d'une LED (lem) et canaux de couleur VKB (0 à 7 par composante).
const (
	ledOff   = 0
	ledOn    = 1
	ledSlow  = 2
	ledFast  = 3
	ledUltra = 4
)

type rgb [3]uint8 // rouge, vert, bleu de 0 à 7

type ledCfg struct {
	ID    uint8
	Mode  uint8
	Color rgb
}

// bytes : 1 octet d'identifiant puis 24 bits clm|lem|b2|g2|r2|b1|g1|r1 (3 bits chacun), octets inversés.
func (l ledCfg) bytes() []byte {
	v := uint32(0)<<21 | uint32(l.Mode&7)<<18 | uint32(l.Color[2]&7)<<6 | uint32(l.Color[1]&7)<<3 | uint32(l.Color[0]&7)
	return []byte{l.ID, byte(v), byte(v >> 8), byte(v >> 16)}
}

func ledChecksum(n int, buf []byte) []byte {
	chk := uint16(0xFFFF)
	for i := 0; i < (n+1)*3 && i < len(buf); i++ {
		chk ^= uint16(buf[i])
		for b := 0; b < 8; b++ {
			lsb := chk & 1
			chk >>= 1
			if lsb != 0 {
				chk ^= 0xA001
			}
		}
	}
	out := make([]byte, 2)
	binary.LittleEndian.PutUint16(out, chk)
	return out
}

// ledReport construit le rapport 0x59 pour une liste de LED (12 au maximum).
func ledReport(leds []ledCfg, size int) []byte {
	body := make([]byte, 2, 3+4*len(leds))
	rand.Read(body)
	body = append(body, byte(len(leds)))
	for _, l := range leds {
		body = append(body, l.bytes()...)
	}
	rep := append([]byte{0x59, 0xA5, 0x0A}, ledChecksum(len(leds), body)...)
	rep = append(rep, body...)
	if size < len(rep) {
		size = len(rep)
	}
	return append(rep, make([]byte, size-len(rep))...)
}

func (d vkbDevice) setLEDs(leds []ledCfg) error {
	if len(leds) > 12 {
		return fmt.Errorf("12 LED au maximum")
	}
	h, err := openHID(d.Path)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	rep := ledReport(leds, d.FeatLen)
	if r, _, e := procHidSetFeature.Call(uintptr(h), uintptr(unsafe.Pointer(&rep[0])), uintptr(len(rep))); r == 0 {
		return e
	}
	return nil
}