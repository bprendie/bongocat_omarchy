package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/bprendie/bongocat_omarchy/internal/host"
)

const version = "0.2.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}

	switch args[0] {
	case "--version", "-version", "version":
		fmt.Println("bongo-cat-omarchy", version)
		return nil
	case "ports":
		return cmdPorts()
	case "inputs":
		return cmdInputs()
	case "doctor":
		if err := cmdPorts(); err != nil {
			fmt.Println(err)
		}
		fmt.Println()
		if err := cmdInputs(); err != nil {
			fmt.Println(err)
		}
		return nil
	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		port := fs.String("port", "", "serial device")
		inputs := multiFlag{}
		fs.Var(&inputs, "input", "keyboard event device; may be repeated")
		idleTimeout := fs.Duration("idle-timeout", time.Second, "typing idle timeout")
		sleepTimeout := fs.Duration("sleep-timeout", time.Minute, "idle delay before IDLE_START")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		cfg := host.Config{
			Port:         *port,
			Inputs:       inputs,
			IdleTimeout:  *idleTimeout,
			SleepTimeout: *sleepTimeout,
		}
		return host.Run(ctx, cfg)
	case "service":
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		exe, _ = filepath.Abs(exe)
		port := ""
		if detected := host.AutoDetectSerialPort(); detected != "" {
			port = " --port " + detected
		}
		fmt.Printf(`[Unit]
Description=Bongo Cat Omarchy host
After=graphical-session.target

[Service]
Type=simple
ExecStart=%s run%s
Restart=on-failure
RestartSec=3

[Install]
WantedBy=default.target
`, exe, port)
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage() {
	fmt.Println(`Usage:
  bongo-cat-omarchy run [--port /dev/serial/by-id/...] [--input /dev/input/eventX]
  bongo-cat-omarchy ports
  bongo-cat-omarchy inputs
  bongo-cat-omarchy doctor
  bongo-cat-omarchy service
  bongo-cat-omarchy --version`)
}

func cmdPorts() error {
	ports := host.ListSerialPorts()
	if len(ports) == 0 {
		return errors.New("no serial ports found")
	}
	for _, port := range ports {
		marker := " "
		if port.LikelyESP32 {
			marker = "*"
		}
		fmt.Printf("%s %s\t%s\n", marker, port.Path, port.Description)
	}
	return nil
}

func cmdInputs() error {
	inputs := host.ListInputDevices()
	if len(inputs) == 0 {
		return errors.New("no input devices found")
	}
	sort.Slice(inputs, func(i, j int) bool {
		return inputs[i].Path < inputs[j].Path
	})
	for _, input := range inputs {
		marker := " "
		if input.LikelyKeyboard {
			marker = "*"
		}
		fmt.Printf("%s %s\t%s\n", marker, input.Path, input.Name)
	}
	return nil
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
