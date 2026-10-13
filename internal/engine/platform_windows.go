//go:build windows

package engine

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func GetSystemMemory() (MemoryInfo, error) {
	var mi MemoryInfo
	out, err := exec.Command("wmic", "ComputerSystem", "get", "TotalPhysicalMemory").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || line == "TotalPhysicalMemory" {
				continue
			}
			mi.Total, _ = strconv.ParseUint(line, 10, 64)
		}
	}
	out, err = exec.Command("wmic", "OS", "get", "FreePhysicalMemory").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || line == "FreePhysicalMemory" {
				continue
			}
			freeKB, _ := strconv.ParseUint(line, 10, 64)
			mi.Free = freeKB * 1024
		}
	}
	mi.Available = mi.Free
	mi.Used = mi.Total - mi.Free
	if mi.Total > 0 {
		mi.UsagePercent = float64(mi.Used) / float64(mi.Total) * 100
	}
	return mi, nil
}

func GetLoad() [3]float64 { return [3]float64{0, 0, 0} }

func ListProcesses() ([]ProcessInfo, error) {
	out, err := exec.Command("wmic", "process", "get", "ProcessId,WorkingSetSize,Name").Output()
	if err != nil {
		return nil, err
	}
	var list []ProcessInfo
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		rss, _ := strconv.ParseUint(fields[1], 10, 64)
		name := fields[2]
		list = append(list, ProcessInfo{
			PID: pid, Name: name, RSS: rss, RSSMB: float64(rss) / 1024 / 1024,
		})
	}
	return list, nil
}

func KillProcess(pid int) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProc := kernel32.NewProc("OpenProcess")
	terminate := kernel32.NewProc("TerminateProcess")
	closeHandle := kernel32.NewProc("CloseHandle")
	const procTerminate = 0x0001
	handle, _, _ := openProc.Call(uintptr(procTerminate), 1, uintptr(pid))
	if handle == 0 {
		return fmt.Errorf("cannot open process %d", pid)
	}
	_, _, err := terminate.Call(handle, 0)
	closeHandle.Call(handle)
	if err != nil && err != syscall.Errno(0) {
		return err
	}
	return nil
}

func PlatformDropCaches() error {
	// Windows: use EmptyWorkingSet via wmic or powershell
	_ = exec.Command("powershell", "-Command", "[System.GC]::Collect()").Run()
	return nil
}

func init() {
	_ = fmt.Sprintf
}
