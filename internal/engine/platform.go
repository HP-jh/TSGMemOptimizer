//go:build !linux && !darwin && !windows

package engine

import "fmt"

func GetSystemMemory() (MemoryInfo, error) {
	return MemoryInfo{}, fmt.Errorf("unsupported platform")
}

func GetLoad() [3]float64 { return [3]float64{0, 0, 0} }

func ListProcesses() ([]ProcessInfo, error) { return nil, fmt.Errorf("unsupported platform") }

func KillProcess(pid int) error { return fmt.Errorf("unsupported platform") }

func PlatformDropCaches() error { return nil }
