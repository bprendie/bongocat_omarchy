package host

import (
	"context"
	"fmt"
	"time"
)

type Config struct {
	Port         string
	Inputs       []string
	IdleTimeout  time.Duration
	SleepTimeout time.Duration
	ClockFormat  string
}

func Run(ctx context.Context, cfg Config) error {
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = time.Second
	}
	if cfg.SleepTimeout <= 0 {
		cfg.SleepTimeout = time.Minute
	}
	if cfg.ClockFormat == "" {
		cfg.ClockFormat = "24h"
	}

	link, err := waitForSerial(ctx, cfg.Port)
	if err != nil {
		return err
	}
	if link == nil {
		return nil
	}
	defer link.Close()
	fmt.Printf("Connected to ESP32 on %s\n", link.Path())
	sendClockFormat(link, cfg.ClockFormat)

	typing, err := StartTypingMonitor(cfg.Inputs)
	if err != nil {
		return err
	}
	defer typing.Close()

	metrics := NewMetrics()
	fmt.Println("Monitoring started. Press Ctrl+C to stop.")

	lastStats := time.Time{}
	lastTime := time.Time{}
	lastActive := time.Now()
	lastAnimation := ""
	lastAnimationSent := time.Time{}
	idleStartSent := false
	streakActive := false

	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = link.Send("STOP")
			return nil
		case now := <-ticker.C:
			send := func(command string) bool {
				next, ok := sendOrReconnect(ctx, link, cfg.Port, cfg.ClockFormat, command)
				link = next
				return ok
			}

			wpm, active := typing.Snapshot(cfg.IdleTimeout)
			if active {
				lastActive = now
				idleStartSent = false
			}

			if active && wpm > 0 {
				speed := WPMToAnimationSpeed(wpm)
				command := fmt.Sprintf("SPEED:%d", speed)
				commandChanged := command != lastAnimation
				keepaliveDue := now.Sub(lastAnimationSent) >= 1200*time.Millisecond
				canChangeSpeed := now.Sub(lastAnimationSent) >= 450*time.Millisecond
				if keepaliveDue || (commandChanged && canChangeSpeed) {
					if send(command) {
						lastAnimation = command
						lastAnimationSent = now
					}
				}

				wantStreak := wpm >= 65
				if wantStreak != streakActive {
					if wantStreak {
						_ = send("STREAK_ON")
					} else {
						_ = send("STREAK_OFF")
					}
					streakActive = wantStreak
				}
			} else if lastAnimation != "STOP" {
				if send("STOP") {
					lastAnimation = "STOP"
				}
				if streakActive {
					_ = send("STREAK_OFF")
					streakActive = false
				}
			} else if !idleStartSent && now.Sub(lastActive) >= cfg.SleepTimeout {
				if send("IDLE_START") {
					idleStartSent = true
				}
			}

			if now.Sub(lastStats) >= 2*time.Second {
				cpu, ram := metrics.Snapshot()
				if send(fmt.Sprintf("STATS:CPU:%d,RAM:%d,WPM:%d", int(cpu+0.5), int(ram+0.5), int(wpm+0.5))) {
					lastStats = now
				}
			}

			if now.Sub(lastTime) >= 30*time.Second {
				if send("TIME:" + time.Now().Format("15:04")) {
					lastTime = now
				}
			}
		}
	}
}

func waitForSerial(ctx context.Context, port string) (*SerialLink, error) {
	for {
		link, err := OpenSerial(port)
		if err == nil {
			return link, nil
		}
		fmt.Printf("Waiting for ESP32 serial device: %v\n", err)

		timer := time.NewTimer(3 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, nil
		case <-timer.C:
		}
	}
}

func sendOrReconnect(ctx context.Context, link *SerialLink, port string, clockFormat string, command string) (*SerialLink, bool) {
	if err := link.Send(command); err == nil {
		return link, true
	} else {
		fmt.Printf("Serial write failed, reconnecting: %v\n", err)
	}

	_ = link.CloseWithoutStop()
	next, err := waitForSerial(ctx, port)
	if err != nil || next == nil {
		return link, false
	}
	fmt.Printf("Reconnected to ESP32 on %s\n", next.Path())
	sendClockFormat(next, clockFormat)

	if err := next.Send(command); err != nil {
		fmt.Printf("Serial write failed after reconnect: %v\n", err)
		_ = next.CloseWithoutStop()
		return link, false
	}
	return next, true
}

func sendClockFormat(link *SerialLink, clockFormat string) {
	if clockFormat == "12h" {
		_ = link.Send("TIME_FORMAT:12")
	} else {
		_ = link.Send("TIME_FORMAT:24")
	}
	_ = link.Send("TIME:" + time.Now().Format("15:04"))
}

func WPMToAnimationSpeed(wpm float64) int {
	if wpm <= 0 {
		return 500
	}
	maxWPM := 120.0
	minSpeed := 55.0
	maxSpeed := 140.0
	if wpm > maxWPM {
		wpm = maxWPM
	}
	normalized := wpm / maxWPM
	speed := maxSpeed - normalized*(maxSpeed-minSpeed)
	bucket := 10.0
	quantized := int((speed/bucket)+0.5) * int(bucket)
	if quantized < int(minSpeed) {
		return int(minSpeed)
	}
	if quantized > int(maxSpeed) {
		return int(maxSpeed)
	}
	return quantized
}
