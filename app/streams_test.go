package app

import (
	"context"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

func TestOpenTelnetAndSerialSpecs(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err == nil {
			c.Write([]byte("hello\r\n"))
			time.Sleep(200 * time.Millisecond)
			c.Close()
		}
	}()

	var mu sync.Mutex
	states := map[string][]string{}
	svc := NewSessionService(nil, nil, nil, nil, func(name string, data any) {
		if st, ok := data.(SessionState); ok {
			mu.Lock()
			states[st.ID] = append(states[st.ID], st.State)
			mu.Unlock()
		}
	})
	port := ln.Addr().(*net.TCPAddr).Port
	info, err := svc.OpenTarget(context.Background(), "telnet://127.0.0.1:"+strconv.Itoa(port), 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if info.Protocol != ProtocolTelnet || info.Target.Port != port || info.Title != "telnet 127.0.0.1 "+strconv.Itoa(port) {
		t.Fatalf("info = %+v", info)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		got := append([]string(nil), states[info.ID]...)
		mu.Unlock()
		if len(got) >= 2 && got[1] == StateConnected {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("states = %v", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
	svc.Close(info.ID)

	if _, err := svc.OpenTarget(context.Background(), "telnet://", 80, 24); errs.CodeOf(err) != ErrInvalidTelnet {
		t.Errorf("empty telnet spec: %v", err)
	}
	ser, err := svc.OpenTarget(context.Background(), "serial:///nonexistent/tty-shellvane?baud=115200", 80, 24)
	if err != nil || ser.Protocol != ProtocolSerial || ser.Title != "/nonexistent/tty-shellvane 115200 8N1" {
		t.Fatalf("serial = %+v, %v", ser, err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		got := append([]string(nil), states[ser.ID]...)
		mu.Unlock()
		if len(got) >= 2 {
			if got[1] != StateFailed {
				t.Fatalf("missing port: states = %v", got)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("states = %v", got)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
