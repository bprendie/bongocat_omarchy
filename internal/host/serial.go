package host

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type SerialPort struct {
	Path        string
	Description string
	LikelyESP32 bool
}

type SerialLink struct {
	file     *os.File
	path     string
	lastSend time.Time
}

func ListSerialPorts() []SerialPort {
	var ports []SerialPort

	for _, pattern := range []string{"/dev/serial/by-id/*", "/dev/ttyACM*", "/dev/ttyUSB*"} {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			ports = appendIfMissing(ports, describeSerial(path))
		}
	}

	sort.Slice(ports, func(i, j int) bool {
		if ports[i].LikelyESP32 != ports[j].LikelyESP32 {
			return ports[i].LikelyESP32
		}
		return ports[i].Path < ports[j].Path
	})
	return ports
}

func AutoDetectSerialPort() string {
	ports := ListSerialPorts()
	for _, port := range ports {
		if port.LikelyESP32 {
			return port.Path
		}
	}
	if len(ports) == 1 {
		return ports[0].Path
	}
	return ""
}

func OpenSerial(path string) (*SerialLink, error) {
	if path == "" {
		path = AutoDetectSerialPort()
	}
	if path == "" {
		return nil, fmt.Errorf("could not auto-detect ESP32 serial port")
	}

	file, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", path, err)
	}
	if err := configureSerial(int(file.Fd())); err != nil {
		_ = file.Close()
		return nil, err
	}

	link := &SerialLink{file: file, path: path}
	time.Sleep(2 * time.Second)
	_ = link.Send("PING")
	_ = link.Send("TIME:" + time.Now().Format("15:04"))
	_ = link.Send("STATS:CPU:0,RAM:0,WPM:0")
	return link, nil
}

func (l *SerialLink) Path() string {
	return l.path
}

func (l *SerialLink) Send(command string) error {
	elapsed := time.Since(l.lastSend)
	if elapsed < 50*time.Millisecond {
		time.Sleep(50*time.Millisecond - elapsed)
	}
	_, err := l.file.WriteString(command + "\n")
	l.lastSend = time.Now()
	return err
}

func (l *SerialLink) Close() error {
	if l.file != nil {
		_ = l.Send("STOP")
		return l.file.Close()
	}
	return nil
}

func configureSerial(fd int) error {
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}
	t.Iflag = unix.IGNPAR
	t.Oflag = 0
	t.Cflag = unix.B115200 | unix.CS8 | unix.CLOCAL | unix.CREAD
	t.Lflag = 0
	t.Cc[unix.VMIN] = 0
	t.Cc[unix.VTIME] = 10
	t.Ispeed = unix.B115200
	t.Ospeed = unix.B115200
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, t); err != nil {
		return err
	}
	return unix.SetNonblock(fd, false)
}

func describeSerial(path string) SerialPort {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}
	base := filepath.Base(path)
	desc := base
	haystack := strings.ToLower(path + " " + target + " " + base)
	keywords := []string{"1a86", "ch340", "ch341", "cp210", "10c4", "ftdi", "0403", "esp32", "espressif", "303a"}
	likely := false
	for _, keyword := range keywords {
		if strings.Contains(haystack, keyword) {
			likely = true
			break
		}
	}
	if strings.Contains(haystack, "quectel") {
		likely = false
	}
	return SerialPort{Path: path, Description: desc, LikelyESP32: likely}
}

func appendIfMissing(ports []SerialPort, next SerialPort) []SerialPort {
	for _, port := range ports {
		if port.Path == next.Path {
			return ports
		}
	}
	return append(ports, next)
}
