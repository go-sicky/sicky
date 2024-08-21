/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2021 HereweTech Co.LTD
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 * @file net.go
 * @package utils
 * @author Dr.NP <np@herewe.tech>
 * @since 08/20/2024
 */

package utils

import (
	"net"
)

func ObtainIPs() ([]net.IP, error) {
	ret := make([]net.IP, 0)

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPAddr:
				ret = append(ret, v.IP)
			case *net.IPNet:
				ret = append(ret, v.IP)
			}
		}
	}

	return ret, nil
}

func ObtainPreferIP(ipv4Only bool) (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ips := make([][]net.IP, 4)

	// Public
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPAddr:
				ip = v.IP
			case *net.IPNet:
				ip = v.IP
			}

			if ipv4Only && ip.To4() == nil {
				continue
			}

			if ip.IsLoopback() {
				ips[0] = append(ips[0], ip)
			} else if ip.IsPrivate() {
				ips[1] = append(ips[1], ip)
			} else if ip.IsMulticast() {
				ips[2] = append(ips[2], ip)
			} else if !ip.IsUnspecified() {
				ips[3] = append(ips[3], ip)
			}
		}
	}

	if len(ips[3]) > 0 {
		return ips[3][0], nil
	} else if len(ips[2]) > 0 {
		return ips[2][0], nil
	} else if len(ips[1]) > 0 {
		return ips[1][0], nil
	} else if len(ips[0]) > 0 {
		return ips[0][0], nil
	}

	return nil, nil
}

func AddrToIP(addr net.Addr) net.IP {
	switch addr.Network() {
	case "tcp", "tcp4", "tcp6":
		// TCP
		if tcpAddr, ok := addr.(*net.TCPAddr); ok {
			return tcpAddr.IP
		}
	case "udp", "udp4", "udp6":
		// UDP
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			return udpAddr.IP
		}
	case "ip", "ip4", "ip6":
		// IP
		if ipAddr, ok := addr.(*net.IPAddr); ok {
			return ipAddr.IP
		}
	default:
		// Unsupport
	}

	return nil
}

func AddrToPort(addr net.Addr) int {
	switch addr.Network() {
	case "tcp", "tcp4", "tcp6":
		// TCP
		if tcpAddr, ok := addr.(*net.TCPAddr); ok {
			return tcpAddr.Port
		}
	case "udp", "udp4", "udp6":
		// UDP
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			return udpAddr.Port
		}
	default:
		// Unsupport
	}

	return 0
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
