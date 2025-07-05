package provider

import "net/netip"

type DDNSProvider interface {
	Update(ip netip.Addr) error
}
