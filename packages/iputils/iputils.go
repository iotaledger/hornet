package iputils

import (
	"fmt"
	"github.com/pkg/errors"
	"net"
	"strconv"
	"strings"
)

type IP struct {
	net.IP
}

func (ip *IP) IsIPv6() bool {
	return len(ip.To4()) != net.IPv4len
}

func (ip *IP) ToString() string {
	if ip.IsIPv6() {
		return "[" + ip.String() + "]"
	}

	return ip.String()
}

type NeighborIPAddresses struct {
	IPs map[*IP]struct{}
}

func NewNeighborIPAddresses() *NeighborIPAddresses {
	return &NeighborIPAddresses{IPs: make(map[*IP]struct{})}
}

func (ips *NeighborIPAddresses) GetPreferredAddress(preferIPv6 bool) *IP {
	if !preferIPv6 {
		for ip := range ips.IPs {
			if !ip.IsIPv6() {
				return ip
			}
		}
	} else {
		for ip := range ips.IPs {
			if ip.IsIPv6() {
				return ip
			}
		}
	}
	// it's a map/set
	for ip := range ips.IPs {
		return ip
	}
	return nil
}

func (ips *NeighborIPAddresses) Add(ip *IP) {
	ips.IPs[ip] = struct{}{}
}

func (ips *NeighborIPAddresses) Remove(ip *IP) {
	delete(ips.IPs, ip)
}

func (ips *NeighborIPAddresses) Len() int {
	return len(ips.IPs)
}

// OriginAddress represents a tuple of a IP or hostname, port and IPv6 preference
type OriginAddress struct {
	Addr       string
	Port       uint16
	PreferIPv6 bool
}

func (ra *OriginAddress) String() string {
	return fmt.Sprintf("%s:%d", ra.Addr, ra.Port)
}

var ErrOriginAddrInvalidAddrChunk = errors.New("invalid address chunk in origin address")
var ErrOriginAddrInvalidPort = errors.New("invalid port in origin address")

func ParseOriginAddress(s string) (*OriginAddress, error) {
	addressChunks := strings.Split(s, ":")
	if len(addressChunks) <= 1 {
		return nil, ErrOriginAddrInvalidAddrChunk
	}

	addr := strings.Join(addressChunks[:len(addressChunks)-1], ":")
	portInt, err := strconv.Atoi(addressChunks[len(addressChunks)-1])
	if err != nil {
		return nil, ErrOriginAddrInvalidPort
	}
	port := uint16(portInt)
	return &OriginAddress{Addr: addr, Port: port}, nil
}
