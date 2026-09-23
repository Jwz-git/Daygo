package favicon

import (
	"context"
	"net"
	"testing"
)

func TestNormalizeHost(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "bare domain", in: "bilibili.com", want: "bilibili.com"},
		{name: "uppercase trimmed", in: "  GitHub.com ", want: "github.com"},
		{name: "strips www", in: "www.example.com", want: "example.com"},
		{name: "strips path", in: "developer.apple.com/xcode", want: "developer.apple.com"},
		{name: "strips scheme", in: "https://example.com/x", want: "example.com"},
		{name: "strips port", in: "example.com:8443", want: "example.com"},
		{name: "single label gets .com", in: "pinterest", want: "pinterest.com"},
		{name: "empty", in: "", wantErr: true},
		{name: "display name with space", in: "Microsoft Edge", wantErr: true},
		{name: "ip literal rejected", in: "127.0.0.1", wantErr: true},
		{name: "ipv6 literal rejected", in: "[::1]", wantErr: true},
		{name: "numeric tld rejected", in: "10.0.0.1", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeHost(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeHost(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeHost(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeHost(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsPublicIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"127.0.0.1", false},
		{"10.0.0.5", false},
		{"192.168.1.1", false},
		{"172.16.0.1", false},
		{"169.254.0.1", false}, // link-local
		{"0.0.0.0", false},     // unspecified
		{"0.1.2.3", false},     // "this network" 0.0.0.0/8
		{"100.64.0.1", false},  // carrier-grade NAT
		{"100.127.255.1", false},
		{"198.18.0.5", false}, // benchmarking
		{"192.0.0.1", false},  // IETF protocol assignments
		{"240.0.0.1", false},  // reserved / class E
		{"255.255.255.255", false},
		{"100.128.0.1", true}, // just past the CGNAT block, public again
		{"::1", false},        // loopback v6
		{"fe80::1", false},    // link-local v6
		{"2606:4700:4700::1111", true},
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Fatalf("bad test ip %q", tc.ip)
		}
		if got := isPublicIP(ip); got != tc.want {
			t.Errorf("isPublicIP(%s) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

func TestGuardedDialerRefusesNonPublic(t *testing.T) {
	d := &guardedDialer{base: &net.Dialer{}}
	// localhost resolves to loopback; the dialer must refuse before connecting.
	_, err := d.DialContext(context.Background(), "tcp", "localhost:80")
	if err == nil {
		t.Fatal("expected guardedDialer to refuse localhost, got nil error")
	}
}
