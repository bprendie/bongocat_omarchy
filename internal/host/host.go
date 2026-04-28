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
}

func Run(ctx context.Context, cfg Config) error {
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = time.Second
	}
	if cfg.SleepTimeout <= 0 {
		cfg.SleepTimeout = time.Minute
	}

	link, err := OpenSerial(cfg.Port)
	if err != nil {
		return err
	}
	defer link.Close()
	fmt.Printf("Connected to ESP32 on %s\n", link.Path())

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
					_ = link.Send(command)
					lastAnimation = command
					lastAnimationSent = now
				}

				wantStreak := wpm >= 65
				if wantStreak != streakActive {
					if wantStreak {
						_ = link.Send("STREAK_ON")
					} else {
						_ = link.Send("STREAK_OFF")
					}
					streakActive = wantStreak
				}
			} else if lastAnimation != "STOP" {
				_ = link.Send("STOP")
				lastAnimation = "STOP"
				if streakActive {
					_ = link.Send("STREAK_OFF")
					streakActive = false
				}
			} else if !idleStartSent && now.Sub(lastActive) >= cfg.SleepTimeout {
				_ = link.Send("IDLE_START")
				idleStartSent = true
			}

			if now.Sub(lastStats) >= 2*time.Second {
				cpu, ram := metrics.Snapshot()
				_ = link.Send(fmt.Sprintf("STATS:CPU:%d,RAM:%d,WPM:%d", int(cpu+0.5), int(ram+0.5), int(wpm+0.5)))
				lastStats = now
			}

			if now.Sub(lastTime) >= 30*time.Second {
				_ = link.Send("TIME:" + time.Now().Format("15:04"))
				lastTime = now
			}
		}
	}
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
