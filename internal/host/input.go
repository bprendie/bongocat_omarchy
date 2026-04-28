package host

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	evKey    = 0x01
	keyPress = 0x01
	keyA     = 30
	keySpace = 57
)

type InputDevice struct {
	Path           string
	Name           string
	LikelyKeyboard bool
}

type TypingMonitor struct {
	mu          sync.Mutex
	keystrokes  []time.Time
	lastKey     time.Time
	active      bool
	previousWPM float64
	done        chan struct{}
	files       []*os.File
}

func ListInputDevices() []InputDevice {
	matches, _ := filepath.Glob("/dev/input/event*")
	devices := make([]InputDevice, 0, len(matches))
	for _, path := range matches {
		name := inputDeviceName(path)
		devices = append(devices, InputDevice{
			Path:           path,
			Name:           name,
			LikelyKeyboard: hasKeys(path, keyA, keySpace),
		})
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Path < devices[j].Path })
	return devices
}

func StartTypingMonitor(paths []string) (*TypingMonitor, error) {
	if len(paths) == 0 {
		for _, dev := range ListInputDevices() {
			if dev.LikelyKeyboard {
				paths = append(paths, dev.Path)
			}
		}
	}

	monitor := &TypingMonitor{done: make(chan struct{})}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			monitor.Close()
			return nil, err
		}
		monitor.files = append(monitor.files, file)
		go monitor.readLoop(file)
	}
	return monitor, nil
}

func (m *TypingMonitor) Close() {
	select {
	case <-m.done:
	default:
		close(m.done)
	}
	for _, file := range m.files {
		_ = file.Close()
	}
}

func (m *TypingMonitor) Snapshot(idleTimeout time.Duration) (float64, bool) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active && now.Sub(m.lastKey) > idleTimeout {
		m.active = false
		m.previousWPM = 0
		m.keystrokes = nil
		return 0, false
	}
	if !m.active {
		return 0, false
	}
	if len(m.keystrokes) < 2 {
		return 0, true
	}
	start := 0
	if len(m.keystrokes) > 8 {
		start = len(m.keystrokes) - 8
	}
	recent := m.keystrokes[start:]
	span := now.Sub(recent[0]).Seconds()
	if span < 0.4 {
		return 0, true
	}
	raw := (float64(len(recent)) / 5.0) / (span / 60.0)
	wpm := raw
	if m.previousWPM > 0 {
		wpm = m.previousWPM*0.6 + raw*0.4
	}
	if wpm > 200 {
		wpm = 200
	}
	m.previousWPM = wpm
	return wpm, true
}

func (m *TypingMonitor) readLoop(file *os.File) {
	buf := make([]byte, 24)
	for {
		select {
		case <-m.done:
			return
		default:
		}
		_, err := file.Read(buf)
		if err != nil {
			return
		}
		eventType := binary.LittleEndian.Uint16(buf[16:18])
		code := binary.LittleEndian.Uint16(buf[18:20])
		value := int32(binary.LittleEndian.Uint32(buf[20:24]))
		if eventType == evKey && value == keyPress && isTypingKey(int(code)) {
			m.recordKey()
		}
	}
}

func (m *TypingMonitor) recordKey() {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keystrokes = append(m.keystrokes, now)
	if len(m.keystrokes) > 50 {
		m.keystrokes = m.keystrokes[len(m.keystrokes)-50:]
	}
	m.lastKey = now
	m.active = true
}

func isTypingKey(code int) bool {
	if code <= 0 {
		return false
	}
	ignored := map[int]bool{
		1: true, 14: true, 15: true, 29: true, 42: true, 54: true,
		56: true, 97: true, 100: true, 102: true, 103: true, 104: true,
		105: true, 106: true, 107: true, 108: true, 109: true, 110: true,
		111: true, 119: true, 125: true, 126: true,
	}
	if ignored[code] || (code >= 59 && code <= 88) {
		return false
	}
	return code < 256
}

func inputDeviceName(path string) string {
	parts := strings.Split(path, "/")
	namePath := filepath.Join("/sys/class/input", parts[len(parts)-1], "device/name")
	data, err := os.ReadFile(namePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func hasKeys(path string, keys ...int) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	bits := make([]byte, 96)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, file.Fd(), uintptr(evIOCGBit(evKey, len(bits))), uintptr(unsafe.Pointer(&bits[0])))
	if errno != 0 {
		return false
	}
	for _, key := range keys {
		if !testBit(bits, key) {
			return false
		}
	}
	return true
}

func evIOCGBit(ev, length int) uint {
	const iocRead = 2
	return uint((iocRead << 30) | (length << 16) | ('E' << 8) | (0x20 + ev))
}

func testBit(bits []byte, bit int) bool {
	idx := bit / 8
	if idx >= len(bits) {
		return false
	}
	return bits[idx]&(1<<uint(bit%8)) != 0
}
