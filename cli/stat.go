package cli

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/sensors"
	"nhatp.com/go/nestor/infra/fs"
)

type Stat struct {
	Time time.Time

	CPUCount   int     // logical cores; divisor for the load averages
	CPUPercent float64 // 0-100, aggregate across cores
	Load1      float64
	Load5      float64
	Load15     float64

	MemTotal   uint64
	MemUsed    uint64
	MemPercent float64

	DiskTotal             uint64
	DiskUsed              uint64
	DiskPercent           float64
	DiskTotalBytesWritten *uint64

	TempC *float64 // nil when unavailable (e.g. macOS)
	Power *float64 // nil when unavailable
}

// Load1PerCore returns the 1-minute load average normalized by core count,
// where 1.0 means saturated. It returns 0 if the core count is unknown.
func (s *Stat) Load1PerCore() float64 {
	if s.CPUCount == 0 {
		return 0
	}
	return s.Load1 / float64(s.CPUCount)
}

// Load5PerCore returns the 1-minute load average normalized by core count,
// where 1.0 means saturated. It returns 0 if the core count is unknown.
func (s *Stat) Load5PerCore() float64 {
	if s.CPUCount == 0 {
		return 0
	}
	return s.Load5 / float64(s.CPUCount)
}

// Load15PerCore returns the 1-minute load average normalized by core count,
// where 1.0 means saturated. It returns 0 if the core count is unknown.
func (s *Stat) Load15PerCore() float64 {
	if s.CPUCount == 0 {
		return 0
	}
	return s.Load15 / float64(s.CPUCount)
}

// CollectStat samples host resources. The CPU reading blocks for interval.
func CollectStat(ctx context.Context, path string, interval time.Duration) (*Stat, error) {
	s := &Stat{Time: time.Now()}

	if n, err := cpu.CountsWithContext(ctx, true); err == nil {
		s.CPUCount = n
	}

	pct, err := cpu.PercentWithContext(ctx, interval, false)
	if err != nil {
		return nil, fmt.Errorf("cpu percent: %w", err)
	}
	if len(pct) > 0 {
		s.CPUPercent = pct[0]
	}

	if avg, err := load.AvgWithContext(ctx); err == nil {
		s.Load1, s.Load5, s.Load15 = avg.Load1, avg.Load5, avg.Load15
	}

	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("virtual memory: %w", err)
	}
	s.MemTotal, s.MemUsed, s.MemPercent = vm.Total, vm.Used, vm.UsedPercent

	du, err := disk.UsageWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("disk usage %s: %w", path, err)
	}
	s.DiskTotal, s.DiskUsed, s.DiskPercent = du.Total, du.Used, du.UsedPercent

	s.TempC = collectTemperature(ctx)
	s.Power = collectPower(ctx)
	s.DiskTotalBytesWritten = collectTotalByteWritten(ctx)
	return s, nil
}

// collectTemperature returns the CPU temperature, or nil if the platform
// does not expose one.
func collectTemperature(ctx context.Context) *float64 {
	if runtime.GOOS != "linux" {
		return nil
	}
	temps, err := sensors.TemperaturesWithContext(ctx)
	if err != nil || len(temps) == 0 {
		return nil
	}
	for _, t := range temps {
		if t.SensorKey == "cpu_thermal" && t.Temperature > 0 {
			return new(t.Temperature)
		}
	}
	return new(temps[0].Temperature)
}

// collectPower returns board power in watts, or nil if unavailable.
func collectPower(ctx context.Context) *float64 {
	if runtime.GOOS != "linux" {
		return nil
	}

	out, err := exec.CommandContext(ctx, "vcgencmd", "pmic_read_adc").Output()
	if err != nil {
		return nil
	}

	amps, volts := map[string]float64{}, map[string]float64{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		// e.g. "VDD_CORE_A current(7)=1.04481000A"
		f := strings.Fields(sc.Text())
		if len(f) != 2 {
			continue
		}
		name := f[0]
		_, val, ok := strings.Cut(f[1], "=")
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimRight(val, "AV"), 64)
		if err != nil {
			continue
		}
		switch {
		case strings.HasSuffix(name, "_A"):
			amps[strings.TrimSuffix(name, "_A")] = v
		case strings.HasSuffix(name, "_V"):
			volts[strings.TrimSuffix(name, "_V")] = v
		}
	}

	var w float64
	for rail, a := range amps {
		w += a * volts[rail] // rails without a voltage contribute 0
	}
	return &w
}

// collectTotalByteWritten returns the total byte written, or nil if the platform
// does not expose one.
func collectTotalByteWritten(ctx context.Context) *uint64 {
	if runtime.GOOS != "linux" {
		return nil
	}
	ts, err := fs.AtomicReadFile("/var/lib/mmc-writes/total")
	if err != nil {
		return nil
	}
	total, err := strconv.Atoi(strings.TrimSpace(string(ts)))
	if err != nil {
		return nil
	}
	return new(uint64(total))
}
