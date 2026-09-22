package inventory

import (
	"net"
	"sort"
)

// CollectNICs returns network interfaces using the standard library, which works
// on all three target operating systems. Interface names are OS-native and do
// not correspond across platforms; MAC is the stable identifier.
func CollectNICs() []NIC {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil
	}
	nics := make([]NIC, 0, len(ifs))
	for _, in := range ifs {
		if len(in.HardwareAddr) == 0 {
			continue // virtual/loopback pseudo-interfaces have no MAC
		}
		nic := NIC{
			Name: in.Name,
			MAC:  in.HardwareAddr.String(),
			MTU:  in.MTU,
			Up:   in.Flags&net.FlagUp != 0,
		}
		addrs, err := in.Addrs()
		if err == nil {
			for _, a := range addrs {
				ip := a.String()
				if ip == "" {
					continue
				}
				nic.IPs = append(nic.IPs, ip)
			}
			sort.Strings(nic.IPs)
		}
		nics = append(nics, nic)
	}
	sort.Slice(nics, func(i, j int) bool { return nics[i].Name < nics[j].Name })
	return nics
}

// DedupeSoftware removes repeated program entries. The same MSI product is often
// registered under several uninstall keys; ProductCode deduplicates those, and
// name+version catches non-MSI duplicates.
func DedupeSoftware(in []Software) []Software {
	byProduct := make(map[string]struct{})
	byNameVer := make(map[string]struct{})
	out := make([]Software, 0, len(in))
	for _, s := range in {
		if s.Name == "" {
			continue
		}
		if s.ProductCode != "" {
			if _, seen := byProduct[s.ProductCode]; seen {
				continue
			}
			byProduct[s.ProductCode] = struct{}{}
		}
		key := s.Name + "\x00" + s.Version + "\x00" + s.Publisher
		if _, seen := byNameVer[key]; seen {
			continue
		}
		byNameVer[key] = struct{}{}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
