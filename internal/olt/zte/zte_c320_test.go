package zte

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestValidateReadOnlyCommand(t *testing.T) {
	allowed := []string{
		"show system-group",
		"show card",
		"show gpon onu state",
		"show pon power onu-rx gpon-onu_1/2/1:1",
		"terminal length 0",
		"exit",
	}
	for _, cmd := range allowed {
		if err := ValidateReadOnlyCommand(cmd); err != nil {
			t.Errorf("expected %q allowed, got error: %v", cmd, err)
		}
	}

	blocked := []string{
		"",
		"configure terminal",
		"reboot",
		"write",
		"interface gpon-olt_1/1/1",
		"show card; reboot",
		"show card | grep x",
		"show card\nconfigure terminal",
	}
	for _, cmd := range blocked {
		if err := ValidateReadOnlyCommand(cmd); err == nil {
			t.Errorf("expected %q blocked, got nil", cmd)
		}
	}
}

func TestRebootONU_BlockedInStrictReadOnly(t *testing.T) {
	adapter := New(Config{Host: "192.0.2.10", Port: 23, Username: "ro", Password: "ro"})
	err := adapter.RebootONU(context.Background(), "1/2/1:1")
	if !errors.Is(err, ErrReadOnlyMode) {
		t.Fatalf("expected ErrReadOnlyMode, got %v", err)
	}
}

func startMockZTEServer(t *testing.T) (host string, port int, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock telnet: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				r := bufio.NewReader(c)
				// Send Telnet DO ECHO + Username prompt
				_, _ = c.Write([]byte{255, 253, 1})
				_, _ = c.Write([]byte("Username:"))
				_, _ = r.ReadString('\n')
				_, _ = c.Write([]byte("Password:"))
				_, _ = r.ReadString('\n')
				_, _ = c.Write([]byte("\r\nOLT-LAB-01#"))

				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					cmd := strings.TrimSpace(line)
					switch {
					case cmd == "exit":
						return
					case cmd == "terminal length 0":
						_, _ = c.Write([]byte("\r\nOLT-LAB-01#"))
					case cmd == "show system-group":
						_, _ = c.Write([]byte(
							"\r\nSystem Description: C320 Version V2.1.0 Software\r\n" +
								"Started before: 12 days, 4 hours\r\n" +
								"System name:  OLT-LAB-01\r\n" +
								"Location: Lab DC\r\nOLT-LAB-01#",
						))
					case cmd == "show card":
						_, _ = c.Write([]byte(
							"\r\n1 1 1 GTGH GTGHG 16 V1.0.0 V2.1.0 INSERVICE\r\n" +
								"1 1 2 SMXA SMXA 3 V1.0.0 V2.1.0 STANDBY\r\nOLT-LAB-01#",
						))
					case cmd == "show gpon onu state":
						_, _ = c.Write([]byte(
							"\r\nOnuIndex   Admin State  OMCC State  Phase State  Channel\r\n" +
								"--------------------------------------------------------------\r\n" +
								"1/1/1:1     enable       enable      working      1(GPON)\r\n" +
								"1/1/1:2     enable       disable     LOS          1(GPON)\r\n" +
								"1/1/1:3     enable       disable     DyingGasp    1(GPON)\r\n" +
								"1/1/1:4     enable       disable     OffLine      1(GPON)\r\nOLT-LAB-01#",
						))
					case strings.HasPrefix(cmd, "show gpon onu state gpon-onu_"):
						_, _ = c.Write([]byte("\r\n1/1/1:1 enable enable working 1(GPON)\r\nOLT-LAB-01#"))
					case strings.HasPrefix(cmd, "show pon power onu-rx"):
						_, _ = c.Write([]byte("\r\nOnu Rx optical power: -19.450(dbm)\r\nOLT-LAB-01#"))
					default:
						_, _ = c.Write([]byte("\r\n%Error\r\nOLT-LAB-01#"))
					}
				}
			}(conn)
		}
	}()
	tcpAddr := ln.Addr().(*net.TCPAddr)
	return tcpAddr.IP.String(), tcpAddr.Port, func() {
		_ = ln.Close()
		<-done
	}
}

func TestZteC320Adapter_MockTelnetReadOnly(t *testing.T) {
	host, port, stop := startMockZTEServer(t)
	defer stop()

	adapter := New(Config{
		Host:     host,
		Port:     port,
		Username: "ro",
		Password: "ro",
		Timeout:  5 * time.Second,
	})

	ctx := context.Background()
	if err := adapter.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}

	sum, err := adapter.GetSummary(ctx)
	if err != nil {
		t.Fatalf("GetSummary error: %v", err)
	}
	if !sum.ReadOnly || sum.Model != "ZTE C320" || sum.Firmware != "V2.1.0" {
		t.Fatalf("unexpected summary header: %+v", sum)
	}
	if sum.CardsTotal != 2 || sum.CardsInService != 1 {
		t.Fatalf("unexpected cards: %+v", sum)
	}
	if sum.TotalONUs != 4 || sum.OnlineONUs != 1 || sum.LosONUs != 1 || sum.DyingGaspONUs != 1 || sum.OfflineONUs != 1 {
		t.Fatalf("unexpected ONU counts: %+v", sum)
	}

	onus, err := adapter.ListONUs(ctx)
	if err != nil || len(onus) != 4 {
		t.Fatalf("ListONUs got %d err=%v", len(onus), err)
	}

	onu, err := adapter.GetONU(ctx, "1/1/1:1")
	if err != nil {
		t.Fatalf("GetONU error: %v", err)
	}
	if onu.Status != "online" || onu.RxPower != -19.45 {
		t.Fatalf("unexpected ONU detail: %+v", onu)
	}

	if _, err := adapter.GetONU(ctx, "1/1/1:1; reboot"); err == nil {
		t.Fatal("expected invalid onu_id injection to fail")
	}
}
