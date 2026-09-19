//go:build darwin

package favicon

import "testing"

func TestParseScutilProxy(t *testing.T) {
	out := `<dictionary> {
  HTTPEnable : 1
  HTTPProxy : 127.0.0.1
  HTTPPort : 7890
  HTTPSEnable : 1
  HTTPSProxy : 127.0.0.1
  HTTPSPort : 7890
  ProxyAutoConfigEnable : 0
}`
	fields := parseScutilProxy(out)
	if fields["HTTPSEnable"] != "1" {
		t.Fatalf("HTTPSEnable = %q, want 1", fields["HTTPSEnable"])
	}
	if fields["HTTPSProxy"] != "127.0.0.1" {
		t.Fatalf("HTTPSProxy = %q, want 127.0.0.1", fields["HTTPSProxy"])
	}
	if fields["HTTPSPort"] != "7890" {
		t.Fatalf("HTTPSPort = %q, want 7890", fields["HTTPSPort"])
	}
}
