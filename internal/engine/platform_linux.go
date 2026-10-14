//go:build linux

package engine

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func GetSystemMemory() (MemoryInfo, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemoryInfo{}, err
	}
	defer f.Close()

	var mi MemoryInfo
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		val *= 1024
		switch key {
		case "MemTotal":
			mi.Total = val
		case "MemFree":
			mi.Free = val
		case "MemAvailable":
			mi.Available = val
		case "Buffers":
			mi.Buffers = val
		case "Cached":
			mi.Cached = val
		case "SwapTotal":
			mi.SwapTotal = val
		case "SwapFree":
			var swapFree uint64
			swapFree = val
			mi.SwapUsed = mi.SwapTotal - swapFree
		}
	}
	mi.Used = mi.Total - mi.Available
	if mi.Total > 0 {
		mi.UsagePercent = float64(mi.Used) / float64(mi.Total) * 100
	}
	return mi, scanner.Err()
}

func GetLoad() [3]float64 {
	var l [3]float64
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return l
	}
	parts := strings.Fields(string(data))
	if len(parts) >= 3 {
		l[0], _ = strconv.ParseFloat(parts[0], 64)
		l[1], _ = strconv.ParseFloat(parts[1], 64)
		l[2], _ = strconv.ParseFloat(parts[2], 64)
	}
	return l
}

func ListProcesses() ([]ProcessInfo, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var list []ProcessInfo
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		p, err := readProc(pid)
		if err != nil {
			continue
		}
		list = append(list, p)
	}
	return list, nil
}

func readProc(pid int) (ProcessInfo, error) {
	var p ProcessInfo
	p.PID = pid
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return p, err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Name:") {
			p.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		} else if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseUint(fields[1], 10, 64)
				p.RSS = v * 1024
				p.RSSMB = float64(v) / 1024
			}
		}
	}
	cmdline, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	p.Command = strings.ReplaceAll(string(cmdline), "\x00", " ")
	return p, nil
}

func KillProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

func PlatformDropCaches() error {
	// requires root; ignore error if not permitted
	_ = exec.Command("sync").Run()
	return os.WriteFile("/proc/sys/vm/drop_caches", []byte("3"), 0644)
}

func init() {
	_ = filepath.Join
}
