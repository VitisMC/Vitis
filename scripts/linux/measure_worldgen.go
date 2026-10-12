//go:build ignore

// Command measure_worldgen records Linux child-process CPU time and peak RSS as JSON.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// processMetrics includes setup and shutdown, independently of benchmark timing.
type processMetrics struct {
	UserSeconds   float64        `json:"user_seconds"`
	SystemSeconds float64        `json:"system_seconds"`
	PeakRSSKiB    int64          `json:"peak_rss_kib"`
	WallSeconds   float64        `json:"wall_seconds"`
	Success       bool           `json:"success"`
	Samples       []systemSample `json:"samples,omitempty"`
}

// systemSample records Linux observations; unavailable sensors remain zero.
type systemSample struct {
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	RSSKiB         int64   `json:"rss_kib"`
	MeanCPUKHz     int64   `json:"mean_cpu_khz"`
	CPUMilliC      int64   `json:"cpu_milli_celsius"`
}

// observe records bounded ten-second samples until the child exits.
func observe(pid int, started time.Time, stop <-chan struct{}, result chan<- []systemSample) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	samples := make([]systemSample, 0, 540)
	for {
		select {
		case <-stop:
			result <- samples
			return
		case <-ticker.C:
			if len(samples) < 10000 {
				samples = append(samples, readSystemSample(pid, time.Since(started).Seconds()))
			}
		}
	}
}

// readSystemSample reads process RSS and optional frequency and temperature sensors.
func readSystemSample(pid int, elapsed float64) systemSample {
	sample := systemSample{ElapsedSeconds: elapsed}
	if status, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid)); err == nil {
		for _, line := range strings.Split(string(status), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "VmRSS:" {
				sample.RSSKiB, _ = strconv.ParseInt(fields[1], 10, 64)
				break
			}
		}
	}
	frequencies, _ := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_cur_freq")
	var count int64
	for _, path := range frequencies {
		if value := readSensor(path); value > 0 {
			sample.MeanCPUKHz += value
			count++
		}
	}
	if count > 0 {
		sample.MeanCPUKHz /= count
	}
	sensors, _ := filepath.Glob("/sys/class/hwmon/hwmon*/name")
	for _, path := range sensors {
		name, err := os.ReadFile(path)
		if err != nil || strings.TrimSpace(string(name)) != "k10temp" && strings.TrimSpace(string(name)) != "coretemp" {
			continue
		}
		temperatures, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "temp*_input"))
		for _, temperature := range temperatures {
			sample.CPUMilliC = max(sample.CPUMilliC, readSensor(temperature))
		}
	}
	return sample
}

// readSensor returns a numeric sysfs value, or zero when the sensor is unavailable.
func readSensor(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return value
}

// main forwards benchmark output and preserves a failing child's exit status.
func main() {
	if err := measure(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// measure writes exact child resource usage after a successful start, including failed exits.
func measure(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: measure_worldgen output.json program [arguments...]")
	}
	command := exec.Command(args[1], args[2:]...)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	started := time.Now()
	if err := command.Start(); err != nil {
		return fmt.Errorf("start benchmark: %w", err)
	}
	stop, observations := make(chan struct{}), make(chan []systemSample)
	go observe(command.Process.Pid, started, stop, observations)
	runErr := command.Wait()
	close(stop)
	samples := <-observations
	usage := command.ProcessState.SysUsage().(*syscall.Rusage)
	metrics := processMetrics{
		UserSeconds:   command.ProcessState.UserTime().Seconds(),
		SystemSeconds: command.ProcessState.SystemTime().Seconds(),
		PeakRSSKiB:    usage.Maxrss,
		WallSeconds:   time.Since(started).Seconds(),
		Success:       runErr == nil,
		Samples:       samples,
	}
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("encode metrics: %w", err)
	}
	if err := os.WriteFile(args[0], append(data, '\n'), 0600); err != nil {
		return fmt.Errorf("write metrics: %w", err)
	}
	if runErr != nil {
		return fmt.Errorf("benchmark: %w", runErr)
	}
	return nil
}
