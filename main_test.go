package main

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"powerusagecollection/internal/zeroconf"
)

func TestPickIPv4(t *testing.T) {
	entry := &zeroconf.ServiceEntry{AddrIPv4: []net.IP{net.ParseIP("192.168.1.5")}}
	if got := pickIPv4(entry); got != "192.168.1.5" {
		t.Fatalf("expected IPv4 address, got %q", got)
	}
}

func TestPickIPv4FallsBackToIPv6(t *testing.T) {
	entry := &zeroconf.ServiceEntry{AddrIPv6: []net.IP{net.ParseIP("fe80::1")}}
	if got := pickIPv4(entry); got != "[fe80::1]" {
		t.Fatalf("expected IPv6 wrapped value, got %q", got)
	}
}

func TestPickIPv4ReturnsEmptyWhenNoAddresses(t *testing.T) {
	entry := &zeroconf.ServiceEntry{}
	if got := pickIPv4(entry); got != "" {
		t.Fatalf("expected empty address, got %q", got)
	}
}

func TestFirmwareVersion(t *testing.T) {
	entry := &zeroconf.ServiceEntry{Text: []string{"other=value", "FirmwareVersion=1.2.3"}}
	if got := firmwareVersion(entry); got != "1.2.3" {
		t.Fatalf("expected firmware version to be %q, got %q", "1.2.3", got)
	}
}

func TestFirmwareVersionEmptyWhenMissing(t *testing.T) {
	entry := &zeroconf.ServiceEntry{Text: []string{"missing", "noequal"}}
	if got := firmwareVersion(entry); got != "" {
		t.Fatalf("expected empty firmware version, got %q", got)
	}
}

func TestFetchPowerSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"deviceName":"Lamp","currentWatts":12.5,"timestamp":"2024-02-02T15:04:05Z"}`)
	}))
	defer server.Close()

	info, err := fetchPower(server.URL)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if info.DeviceName != "Lamp" || info.CurrentWatts != 12.5 || info.Timestamp != "2024-02-02T15:04:05Z" {
		t.Fatalf("unexpected PowerInfo: %+v", info)
	}
}

func TestFetchPowerNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "oops", http.StatusInternalServerError)
	}))
	defer server.Close()

	if _, err := fetchPower(server.URL); err == nil || !strings.Contains(err.Error(), "unexpected status 500") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestFetchPowerDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "not-json")
	}))
	defer server.Close()

	if _, err := fetchPower(server.URL); err == nil {
		t.Fatal("expected decode error, got nil")
	}
}

func TestHandleEntryListOnly(t *testing.T) {
	entry := &zeroconf.ServiceEntry{
		Instance: "Demo Device",
		HostName: "demo.local.",
		Text:     []string{"firmware=9.9.9"},
	}

	output := captureOutput(func() { handleEntry(entry, true) })

	if !strings.Contains(output, "Demo Device (demo.local)") {
		t.Fatalf("expected device header in output, got %q", output)
	}
	if !strings.Contains(output, "Firmware: 9.9.9") {
		t.Fatalf("expected firmware value in output, got %q", output)
	}
}

func TestHandleEntryNoIPv4(t *testing.T) {
	entry := &zeroconf.ServiceEntry{
		Instance: "NoIP Device",
		HostName: "noip.local.",
	}

	output := captureOutput(func() { handleEntry(entry, false) })

	if !strings.Contains(output, "No IPv4 address available") {
		t.Fatalf("expected no IPv4 message, got %q", output)
	}
}

func captureOutput(fn func()) string {
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	os.Stdout = w

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			panic(err)
		}
		done <- buf.String()
	}()

	fn()

	w.Close()
	os.Stdout = original

	return <-done
}
