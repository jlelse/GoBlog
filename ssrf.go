package main

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"
)

var blockedNetworks = func() []netip.Prefix {
	prefixes := []string{
		// "This network" (RFC 5735)
		"0.0.0.0/8",
		// Private (RFC 1918)
		"10.0.0.0/8",
		// Carrier-grade NAT (RFC 6598)
		"100.64.0.0/10",
		// Loopback (RFC 5735)
		"127.0.0.0/8",
		// Link-local (RFC 3927), includes AWS/GCP/Azure metadata 169.254.169.254
		"169.254.0.0/16",
		// Private (RFC 1918)
		"172.16.0.0/12",
		// IETF protocol assignments (RFC 6890)
		"192.0.0.0/24",
		// Documentation (TEST-NET-1, RFC 5737)
		"192.0.2.0/24",
		// Private (RFC 1918)
		"192.168.0.0/16",
		// Deprecated 6to4 relay (RFC 7526)
		"192.88.99.0/24",
		// Benchmarking (RFC 2544)
		"198.18.0.0/15",
		// Documentation (TEST-NET-2, RFC 5737)
		"198.51.100.0/24",
		// Documentation (TEST-NET-3, RFC 5737)
		"203.0.113.0/24",
		// Multicast (RFC 5771)
		"224.0.0.0/4",
		// Reserved for future use (RFC 6890)
		"240.0.0.0/4",
		// Unspecified address (RFC 4291)
		"::/128",
		// Loopback (RFC 4291)
		"::1/128",
		// Unique local addresses (RFC 4193)
		"fc00::/7",
		// Link-local (RFC 4291)
		"fe80::/10",
		// Multicast (RFC 4291)
		"ff00::/8",
		// Documentation (RFC 3849)
		"2001:db8::/32",
	}
	result := make([]netip.Prefix, 0, len(prefixes))
	for _, p := range prefixes {
		result = append(result, netip.MustParsePrefix(p))
	}
	return result
}()

func isBlockedNetwork(ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, prefix := range blockedNetworks {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func checkSSRFHost(host string) error {
	if ip, err := netip.ParseAddr(host); err == nil {
		if isBlockedNetwork(ip) {
			return fmt.Errorf("blocked IP address %s", ip)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if isBlockedNetwork(ip) {
			return fmt.Errorf("host %q resolves to blocked IP address %s", host, ip)
		}
	}
	return nil
}

func newSSRFGuardDialContext(dial func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if err := checkSSRFHost(host); err != nil {
			return nil, err
		}
		return dial(ctx, network, addr)
	}
}
