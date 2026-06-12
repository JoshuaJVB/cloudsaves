// Package discover finds CloudSave servers on the local network by scanning
// the local /24 subnet(s) for the server's HTTP port and fingerprinting each
// responder via its FastAPI OpenAPI document. This works regardless of Docker
// networking (it hits the published host port), unlike mDNS from a container.
package discover

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Servers scans every private /24 the machine is attached to and returns the
// base URLs (e.g. "http://192.168.1.50:45231") that respond as CloudSave
// servers. perHostTimeout bounds how long each probe waits.
func Servers(port int, perHostTimeout time.Duration) []string {
	client := &http.Client{Timeout: perHostTimeout}

	var (
		mu    sync.Mutex
		found []string
		wg    sync.WaitGroup
	)
	sem := make(chan struct{}, 128) // cap concurrent probes

	for _, base := range localSubnets() {
		for i := 1; i <= 254; i++ {
			host := fmt.Sprintf("%s.%d", base, i)
			wg.Add(1)
			sem <- struct{}{}
			go func(host string) {
				defer wg.Done()
				defer func() { <-sem }()
				if isCloudSave(client, host, port) {
					url := fmt.Sprintf("http://%s:%d", host, port)
					mu.Lock()
					found = append(found, url)
					mu.Unlock()
				}
			}(host)
		}
	}
	wg.Wait()
	sort.Strings(found)
	return found
}

func isCloudSave(c *http.Client, host string, port int) bool {
	resp, err := c.Get(fmt.Sprintf("http://%s:%d/openapi.json", host, port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var doc struct {
		Info struct {
			Title string `json:"title"`
		} `json:"info"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&doc); err != nil {
		return false
	}
	return doc.Info.Title == "CloudSave"
}

// localSubnets returns the "a.b.c" prefixes of each private IPv4 /24 the
// machine has an address on.
func localSubnets() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var bases []string
	seen := map[string]bool{}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip4 := ipnet.IP.To4()
		if ip4 == nil || ip4.IsLoopback() || !ip4.IsPrivate() {
			continue
		}
		base := fmt.Sprintf("%d.%d.%d", ip4[0], ip4[1], ip4[2])
		if !seen[base] {
			seen[base] = true
			bases = append(bases, base)
		}
	}
	return bases
}
