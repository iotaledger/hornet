package utils

import (
	"net"

	"github.com/pkg/errors"
)

func ParseIPNet(entry string) (*net.IPNet, error) {
	_, ipnet, err := net.ParseCIDR(entry)
	if ipnet == nil || err != nil {
		ip := net.ParseIP(entry)
		if ip == nil {
			return nil, errors.New("invalid IP address")
		}

		err = nil
		ipnet = &net.IPNet{}
		if ip4 := ip.To4(); ip4 != nil {
			ipnet.IP = ip4
			ipnet.Mask = net.CIDRMask(net.IPv4len*8, net.IPv4len*8)
		} else {
			ipnet.IP = ip
			ipnet.Mask = net.CIDRMask(net.IPv6len*8, net.IPv6len*8)
		}
	}

	return ipnet, err
}
