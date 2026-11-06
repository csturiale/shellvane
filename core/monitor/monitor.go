// Package monitor reads a server's load, CPU, memory, disk and uptime with
// one short command over SSH (Linux /proc, with fallbacks for macOS and
// BSD), so nothing needs to be installed on the server.
package monitor

import (
	"bufio"
	"strconv"
	"strings"
)

// Script prints sections the parser reads. It uses only POSIX tools.
const Script = `echo '#os'; uname -s
echo '#load'; cat /proc/loadavg 2>/dev/null || sysctl -n vm.loadavg 2>/dev/null
echo '#cpus'; nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null
echo '#stat'; head -1 /proc/stat 2>/dev/null
echo '#mem'; grep -E '^(MemTotal|MemAvailable|SwapTotal|SwapFree):' /proc/meminfo 2>/dev/null || { sysctl -n hw.memsize 2>/dev/null; vm_stat 2>/dev/null; }
echo '#disk'; df -Pk / 2>/dev/null | tail -1
echo '#uptime'; cut -d' ' -f1 /proc/uptime 2>/dev/null || sysctl -n kern.boottime 2>/dev/null
echo '#end'; true`

// Stats is one sample.
type Stats struct {
	OS       string     `json:"os"`
	Load     [3]float64 `json:"load"`
	CPUs     int        `json:"cpus"`
	CPU      float64    `json:"cpu"`      // percent busy since the previous sample; -1 unknown
	MemTotal int64      `json:"memTotal"` // bytes
	MemUsed  int64      `json:"memUsed"`
	Swap     int64      `json:"swap"`
	SwapUsed int64      `json:"swapUsed"`
	Disk     int64      `json:"disk"` // bytes, filesystem of /
	DiskUsed int64      `json:"diskUsed"`
	Uptime   int64      `json:"uptime"` // seconds; 0 unknown
	// cpuTimes are /proc/stat counters, to compute CPU from two samples.
	busy, total uint64
}

// Parse reads the output of Script; prev (may be nil) gives the CPU use
// since that sample.
func Parse(out string, prev *Stats) Stats {
	s := Stats{CPU: -1}
	sections := map[string][]string{}
	cur := ""
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "#") {
			cur = line[1:]
			continue
		}
		if line != "" {
			sections[cur] = append(sections[cur], line)
		}
	}
	first := func(k string) string {
		if v := sections[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	s.OS = first("os")
	// "0.52 0.58 0.59 1/467 12345" or BSD "{ 1.20 1.31 1.45 }".
	load := strings.Fields(strings.Trim(first("load"), "{} "))
	for i := 0; i < 3 && i < len(load); i++ {
		s.Load[i], _ = strconv.ParseFloat(load[i], 64)
	}
	s.CPUs, _ = strconv.Atoi(first("cpus"))
	if f := strings.Fields(first("stat")); len(f) > 4 && f[0] == "cpu" {
		var total uint64
		var vals []uint64
		for _, v := range f[1:] {
			n, _ := strconv.ParseUint(v, 10, 64)
			vals = append(vals, n)
			total += n
		}
		idle := vals[3]
		if len(vals) > 4 {
			idle += vals[4] // iowait
		}
		s.busy, s.total = total-idle, total
		if prev != nil && prev.total > 0 && s.total > prev.total {
			s.CPU = 100 * float64(s.busy-prev.busy) / float64(s.total-prev.total)
		}
	}
	parseMem(sections["mem"], &s)
	if f := strings.Fields(first("disk")); len(f) >= 4 {
		total, _ := strconv.ParseInt(f[1], 10, 64)
		used, _ := strconv.ParseInt(f[2], 10, 64)
		s.Disk, s.DiskUsed = total*1024, used*1024
	}
	up := first("uptime")
	if v, err := strconv.ParseFloat(up, 64); err == nil {
		s.Uptime = int64(v)
	}
	return s
}

func parseMem(lines []string, s *Stats) {
	kb := map[string]int64{}
	var pageSize, free, inactive, speculative int64
	for _, l := range lines {
		if k, v, ok := strings.Cut(l, ":"); ok {
			f := strings.Fields(v)
			if len(f) > 0 {
				n, _ := strconv.ParseInt(strings.TrimSuffix(f[0], "."), 10, 64)
				kb[k] = n
			}
			// macOS vm_stat: "Mach Virtual Memory Statistics: (page size of 16384 bytes)"
			if i := strings.Index(l, "page size of "); i >= 0 {
				pageSize, _ = strconv.ParseInt(strings.Fields(l[i+len("page size of "):])[0], 10, 64)
			}
			continue
		}
		if n, err := strconv.ParseInt(l, 10, 64); err == nil {
			s.MemTotal = n // hw.memsize
		}
	}
	if total, ok := kb["MemTotal"]; ok {
		s.MemTotal = total * 1024
		s.MemUsed = (total - kb["MemAvailable"]) * 1024
		s.Swap = kb["SwapTotal"] * 1024
		s.SwapUsed = (kb["SwapTotal"] - kb["SwapFree"]) * 1024
		return
	}
	if pageSize > 0 && s.MemTotal > 0 {
		free, inactive, speculative = kb["Pages free"], kb["Pages inactive"], kb["Pages speculative"]
		s.MemUsed = s.MemTotal - (free+inactive+speculative)*pageSize
	}
}
