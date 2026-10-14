//go:build darwin

package engine

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func GetSystemMemory() (MemoryInfo, error) {
	var mi MemoryInfo
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err == nil {
		mi.Total, _ = strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	}
	vmOut, err := exec.Command("vm_stat").Output()
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(vmOut)))
		var pageSize uint64 = 4096
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "page size of") {
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					pageSize, _ = strconv.ParseUint(parts[3], 10, 64)
				}
			}
			if strings.HasPrefix(line, "Pages free:") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					v, _ := strconv.ParseUint(strings.TrimSuffix(parts[2], "."), 10, 64)
					mi.Free = v * pageSize
				}
			}
			if strings.HasPrefix(line, "Pages inactive:") {
				parts := strings.Fields(line)
				if len(parts) >= 3 {
					v, _ := strconv.ParseUint(strings.TrimSuffix(parts[2], "."), 10, 64)
					mi.Cached += v * pageSize
				}
			}
		}
	}
	mi.Available = mi.Free + mi.Cached
	mi.Used = mi.Total - mi.Available
	if mi.Total > 0 {
		mi.UsagePercent = float64(mi.Used) / float64(mi.Total) * 100
	}
	return mi, nil
}

func GetLoad() [3]float64 {
	var l [3]float64
	out, err := exec.Command("sysctl", "-n", "vm.loadavg").Output()
	if err != nil {
		return l
	}
	parts := strings.Fields(string(out))
	if len(parts) >= 3 {
		l[0], _ = strconv.ParseFloat(parts[0], 64)
		l[1], _ = strconv.ParseFloat(parts[1], 64)
		l[2], _ = strconv.ParseFloat(parts[2], 64)
	}
	return l
}

func ListProcesses() ([]ProcessInfo, error) {
	out, err := exec.Command("ps", "-axco", "pid,rss,command").Output()
	if err != nil {
		return nil, err
	}
	var list []ProcessInfo
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		rss, _ := strconv.ParseUint(fields[1], 10, 64)
		cmd := strings.Join(fields[2:], " ")
		name := cmd
		if i := strings.LastIndex(cmd, "/"); i >= 0 {
			name = cmd[i+1:]
		}
		list = append(list, ProcessInfo{
			PID: pid, Name: name, RSS: rss * 1024, RSSMB: float64(rss) / 1024,
			Command: cmd,
		})
	}
	return list, nil
}

func KillProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

func PlatformDropCaches() error {
	return exec.Command("purge").Run()
}

func init() {
	_ = fmt.Sprintf
}
