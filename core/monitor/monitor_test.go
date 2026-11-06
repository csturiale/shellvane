package monitor

import "testing"

const linux1 = `#os
Linux
#load
0.52 0.58 0.59 1/467 12345
#cpus
4
#stat
cpu  1000 0 500 8000 500 0 0 0 0 0
#mem
MemTotal:        8000000 kB
MemAvailable:    6000000 kB
SwapTotal:       2000000 kB
SwapFree:        1500000 kB
#disk
/dev/sda1 100000 25000 75000 25% /
#uptime
12345.67
#end
`

const linux2 = `#os
Linux
#stat
cpu  1600 0 600 8300 500 0 0 0 0 0
#end
`

const darwin = `#os
Darwin
#load
{ 1.20 1.31 1.45 }
#cpus
8
#stat
#mem
17179869184
Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                               10000.
Pages active:                            200000.
Pages inactive:                          190000.
Pages speculative:                          4000.
#disk
/dev/disk3s1s1 488245288 9842104 254806432 4% /
#uptime
{ sec = 1700000000, usec = 0 } Mon Nov 14 22:13:20 2023
#end
`

func TestParseLinux(t *testing.T) {
	a := Parse(linux1, nil)
	if a.OS != "Linux" || a.Load != [3]float64{0.52, 0.58, 0.59} || a.CPUs != 4 || a.CPU != -1 {
		t.Fatalf("a = %+v", a)
	}
	if a.MemTotal != 8000000*1024 || a.MemUsed != 2000000*1024 || a.SwapUsed != 500000*1024 {
		t.Fatalf("mem = %+v", a)
	}
	if a.Disk != 100000*1024 || a.DiskUsed != 25000*1024 || a.Uptime != 12345 {
		t.Fatalf("disk/uptime = %+v", a)
	}
	b := Parse(linux2, &a)
	// busy +700 (user 600 + system 100), total +1000 → 70%.
	if b.CPU < 69.9 || b.CPU > 70.1 {
		t.Fatalf("cpu = %v", b.CPU)
	}
}

func TestParseDarwin(t *testing.T) {
	s := Parse(darwin, nil)
	if s.OS != "Darwin" || s.Load[2] != 1.45 || s.CPUs != 8 {
		t.Fatalf("s = %+v", s)
	}
	if s.MemTotal != 17179869184 || s.MemUsed != 17179869184-(10000+190000+4000)*16384 {
		t.Fatalf("mem = %+v", s)
	}
	if s.Disk != 488245288*1024 || s.Uptime != 0 {
		t.Fatalf("disk = %+v", s)
	}
}
