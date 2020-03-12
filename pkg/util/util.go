package util

import (
	"fmt"
	"net"
	"strconv"
)

func IPPortFromString(addr string) (*net.IPAddr, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, 0, fmt.Errorf("bad StatsD listening address: %s", addr)
	}

	if host == "" {
		host = "0.0.0.0"
	}
	ip, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return nil, 0, fmt.Errorf("Unable to resolve %s: %s", host, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 0 || port > 65535 {
		return nil, 0, fmt.Errorf("Bad port %s: %s", portStr, err)
	}

	return ip, port, nil
}

func UDPAddrFromString(addr string) (*net.UDPAddr, error) {
	ip, port, err := IPPortFromString(addr)
	if err != nil {
		return nil, err
	}
	return &net.UDPAddr{
		IP:   ip.IP,
		Port: port,
		Zone: ip.Zone,
	}, nil
}

func TCPAddrFromString(addr string) (*net.TCPAddr, error) {
	ip, port, err := IPPortFromString(addr)
	if err != nil {
		return nil, err
	}
	return &net.TCPAddr{
		IP:   ip.IP,
		Port: port,
		Zone: ip.Zone,
	}, nil
}
