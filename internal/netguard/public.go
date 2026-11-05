package netguard

import "net/netip"

var reserved = func() []netip.Prefix {
	prefixes := []netip.Prefix{}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		prefixes = append(prefixes, netip.MustParsePrefix(raw))
	}
	return prefixes
}()

func Public(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range reserved {
		if prefix.Contains(a) {
			return false
		}
	}
	return true
}
