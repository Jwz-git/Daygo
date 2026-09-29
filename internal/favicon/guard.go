package favicon

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
)

// maxHostLen bounds a hostname before any DNS work. A card's appSites value is
// short in practice; the cap rejects pathological model output early.
const maxHostLen = 253

// NormalizeHost validates and canonicalizes an untrusted site string into a
// bare hostname suitable for a request. It follows Dayflow's normalizedHost:
// a single-label value without a dot becomes "<value>.com". It rejects
// anything that is not a plausible public hostname.
//
// This is a format gate only; connection-time IP filtering (guardedDialer)
// is the actual SSRF defense, because a name can still resolve to a private
// address.
func NormalizeHost(raw string) (string, error) {
	site := strings.TrimSpace(strings.ToLower(raw))
	if site == "" {
		return "", fmt.Errorf("favicon: empty host")
	}
	// Whitespace inside means this is a display name ("Microsoft Edge"), not a
	// host — those never resolve to a favicon and must not be probed.
	if strings.ContainsAny(site, " \t\r\n") {
		return "", fmt.Errorf("favicon: not a host: %q", raw)
	}
	// Accept "example.com/path" and "https://example.com" by extracting host.
	if strings.Contains(site, "://") {
		u, err := url.Parse(site)
		if err != nil || u.Host == "" {
			return "", fmt.Errorf("favicon: unparseable url: %q", raw)
		}
		site = u.Host
	} else if i := strings.IndexByte(site, '/'); i >= 0 {
		site = site[:i]
	}
	// Strip any port and userinfo.
	if h, _, err := net.SplitHostPort(site); err == nil {
		site = h
	}
	site = strings.TrimPrefix(site, "www.")

	if !strings.Contains(site, ".") {
		site += ".com"
	}
	if len(site) > maxHostLen {
		return "", fmt.Errorf("favicon: host too long")
	}
	if !isPlausibleHost(site) {
		return "", fmt.Errorf("favicon: implausible host: %q", raw)
	}
	// A literal IP is never a favicon target and is the classic SSRF vector.
	if net.ParseIP(site) != nil {
		return "", fmt.Errorf("favicon: ip literal rejected: %q", raw)
	}
	return site, nil
}

// isPlausibleHost accepts dotted labels of letters, digits and hyphens with a
// non-numeric TLD. It is deliberately strict: favicon hosts are ordinary
// public domains.
func isPlausibleHost(host string) bool {
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
				return false
			}
		}
	}
	tld := labels[len(labels)-1]
	for _, c := range tld {
		if c >= '0' && c <= '9' {
			return false
		}
	}
	return true
}

// guardedDialer refuses connections to non-public IP ranges. The host name is
// resolved by the base dialer, then every candidate address is checked before
// the socket is used, so a public name that resolves to an internal address
// (DNS rebinding, SSRF) still cannot connect.
type guardedDialer struct {
	base *net.Dialer
}

func (d *guardedDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return nil, fmt.Errorf("favicon: refusing non-public address %s", ip)
		}
	}
	// Dial the vetted address directly to avoid a second lookup that could
	// resolve to a different (unvetted) IP.
	return d.base.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

// reservedIPv4Blocks are special-use ranges the stdlib net.IP predicates do
// not classify as private/loopback/link-local. Left unblocked they defeat the
// SSRF guard: an untrusted name resolving into one of them would still be
// dialed. Sources: RFC 5735 / 6598 / 6890.
var reservedIPv4Blocks = []net.IPNet{
	mustCIDR("0.0.0.0/8"),     // "this network" — 0.0.0.0 reaches localhost on Linux
	mustCIDR("100.64.0.0/10"), // carrier-grade NAT (RFC 6598)
	mustCIDR("192.0.0.0/24"),  // IETF protocol assignments (RFC 6890)
	mustCIDR("198.18.0.0/15"), // benchmarking (RFC 2544)
	mustCIDR("240.0.0.0/4"),   // reserved / class E, incl. 255.255.255.255
}

func mustCIDR(s string) net.IPNet {
	_, block, err := net.ParseCIDR(s)
	if err != nil {
		panic("favicon: bad reserved CIDR " + s + ": " + err.Error())
	}
	return *block
}

// isPublicIP reports whether ip is a routable public address.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsInterfaceLocalMulticast() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		for i := range reservedIPv4Blocks {
			if reservedIPv4Blocks[i].Contains(v4) {
				return false
			}
		}
	}
	return true
}

// readCapped reads up to limit bytes, erroring if the source exceeds it.
func readCapped(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("favicon: response exceeds %d bytes", limit)
	}
	return data, nil
}
