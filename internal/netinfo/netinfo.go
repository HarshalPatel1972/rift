// Package netinfo finds the LAN addresses a phone can reach this PC on.
package netinfo

import (
	"net"
	"sort"
	"strings"
)

// Addr is one candidate IPv4 address and the interface it belongs to.
type Addr struct {
	IP        string `json:"ip"`
	Interface string `json:"interface"`
	Preferred bool   `json:"preferred"`
}

// virtualHints marks adapters a phone almost never shares a network with.
var virtualHints = []string{"vethernet", "virtualbox", "vmware", "hyper-v", "wsl", "docker", "loopback", "tailscale", "zerotier", "vpn", "bluetooth"}

func isVirtual(name string) bool {
	n := strings.ToLower(name)
	for _, h := range virtualHints {
		if strings.Contains(n, h) {
			return true
		}
	}
	return false
}

// outboundIP reports the source address the OS would use for internet
// traffic. Dialing UDP sends no packets; it only consults the routing table.
func outboundIP() string {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// Candidates lists usable IPv4 addresses, best first. The default-route
// address wins, then physical adapters, then virtual ones.
func Candidates() []Addr {
	out := outboundIP()
	var addrs []Addr
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		as, _ := ifc.Addrs()
		for _, a := range as {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipn.IP.To4()
			if ip == nil || ip.IsLinkLocalUnicast() {
				continue
			}
			addrs = append(addrs, Addr{IP: ip.String(), Interface: ifc.Name, Preferred: ip.String() == out})
		}
	}
	rank := func(a Addr) int {
		switch {
		case a.Preferred:
			return 0
		case !isVirtual(a.Interface):
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(addrs, func(i, j int) bool { return rank(addrs[i]) < rank(addrs[j]) })
	if len(addrs) > 0 {
		addrs[0].Preferred = true
	}
	return addrs
}
