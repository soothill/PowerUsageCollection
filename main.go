package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

// PowerInfo models a simple JSON response for current power usage.
// Adjust fields to match your devices' API shape.
type PowerInfo struct {
	DeviceName   string  `json:"deviceName"`
	CurrentWatts float64 `json:"currentWatts"`
	Voltage      float64 `json:"voltage,omitempty"`
	Amperage     float64 `json:"amperage,omitempty"`
	Timestamp    string  `json:"timestamp,omitempty"`
}

func main() {
	listOnly := flag.Bool("list", false, "Only list Matter devices with their name and firmware version")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fmt.Println("Discovering Matter devices via _matter._tcp…")
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolver error: %v\n", err)
		os.Exit(1)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entries {
			handleEntry(entry, *listOnly)
		}
	}()

	if err := resolver.Browse(ctx, "_matter._tcp", "local.", entries); err != nil {
		fmt.Fprintf(os.Stderr, "browse error: %v\n", err)
		os.Exit(1)
	}
	<-ctx.Done()
}

func handleEntry(entry *zeroconf.ServiceEntry, listOnly bool) {
	host := strings.TrimSuffix(entry.HostName, ".")
	addr := pickIPv4(entry)

	fmt.Printf("\nDiscovered: %s (%s)\n", entry.Instance, host)
	if listOnly {
		fw := firmwareVersion(entry)
		if fw == "" {
			fw = "unknown"
		}

		fmt.Printf("  Name: %s\n", entry.Instance)
		fmt.Printf("  Firmware: %s\n", fw)
		return
	}

	if addr == "" {
		fmt.Println("  No IPv4 address available; skipping power query.")
		return
	}

	powerURL := fmt.Sprintf("http://%s:80/api/power", addr)
	fmt.Printf("  Querying: %s\n", powerURL)

	power, err := fetchPower(powerURL)
	if err != nil {
		fmt.Printf("  Power query failed: %v\n", err)
		return
	}

	fmt.Printf("  Current power: %.2f W", power.CurrentWatts)
	if power.Timestamp != "" {
		fmt.Printf(" (timestamp: %s)", power.Timestamp)
	}
	fmt.Println()
}

func pickIPv4(entry *zeroconf.ServiceEntry) string {
	for _, ip := range entry.AddrIPv4 {
		if ip.To4() != nil {
			return ip.String()
		}
	}

	if len(entry.AddrIPv6) > 0 {
		return fmt.Sprintf("[%s]", entry.AddrIPv6[0].String())
	}
	return ""
}

func fetchPower(url string) (*PowerInfo, error) {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("unexpected status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var info PowerInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

func firmwareVersion(entry *zeroconf.ServiceEntry) string {
	for _, txt := range entry.Text {
		parts := strings.SplitN(txt, "=", 2)
		if len(parts) != 2 {
			continue
		}

		switch strings.ToLower(parts[0]) {
		case "fv", "firmware", "firmwareversion", "version":
			return parts[1]
		}
	}

	return ""
}
