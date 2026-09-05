/*
 * The MIT License (MIT)
 *
 * Copyright (c) 2024 HereweTech Co.LTD
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
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strings"
	"unsafe"
)

// ErrNilConnection is a shared utils value.
var ErrNilConnection = errors.New("utils: nil connection")

// Well-known network names shared by the AddrToIP family.
const (
	networkTCP  = "tcp"
	networkTCP4 = "tcp4"
	networkTCP6 = "tcp6"
	networkUDP  = "udp"
	networkUDP4 = "udp4"
	networkUDP6 = "udp6"
)

// ObtainIPs is a utility helper.
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

// ObtainPreferIP is a utility helper.
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

			if ip == nil {
				continue
			}

			if ipv4Only && ip.To4() == nil {
				continue
			}

			switch {
			case ip.IsLoopback():
				ips[0] = append(ips[0], ip)
			case ip.IsPrivate():
				ips[1] = append(ips[1], ip)
			case ip.IsMulticast():
				ips[2] = append(ips[2], ip)
			case !ip.IsUnspecified():
				ips[3] = append(ips[3], ip)
			}
		}
	}

	// Buckets are ordered loopback < private < multicast < public: the
	// first non-empty bucket from the top wins.
	for i := range slices.Backward(ips) {
		if len(ips[i]) > 0 {
			return ips[i][0], nil
		}
	}

	return nil, nil
}

// AddrToIP is a utility helper.
func AddrToIP(addr net.Addr) net.IP {
	switch addr.Network() {
	case networkTCP, networkTCP4, networkTCP6:
		// TCP
		if tcpAddr, ok := addr.(*net.TCPAddr); ok {
			return tcpAddr.IP
		}
	case networkUDP, networkUDP4, networkUDP6:
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

// AddrToPort is a utility helper.
func AddrToPort(addr net.Addr) int {
	switch addr.Network() {
	case networkTCP, networkTCP4, networkTCP6:
		// TCP
		if tcpAddr, ok := addr.(*net.TCPAddr); ok {
			return tcpAddr.Port
		}
	case networkUDP, networkUDP4, networkUDP6:
		// UDP
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			return udpAddr.Port
		}
	default:
		// Unsupport
	}

	return 0
}

// Advertise is a utility helper.
func Advertise(listen, advertise, network string) net.Addr {
	host, port, err := net.SplitHostPort(advertise)
	if err != nil {
		// Parse listen
		if strings.HasPrefix(listen, ":") {
			// Null IPv4 host
			ip, err := ObtainPreferIP(true)
			if err != nil || ip == nil {
				return nil
			}

			listen = ip.String() + listen
		}

		host, port, err = net.SplitHostPort(listen)
		if err != nil {
			return nil
		}
	}

	switch network {
	case networkTCP, networkTCP4, networkTCP6:
		// TCP
		addr, err := net.ResolveTCPAddr(network, net.JoinHostPort(host, port))
		if err != nil {
			return nil
		}

		return addr
	case networkUDP, networkUDP4, networkUDP6:
		// UDP
		addr, err := net.ResolveUDPAddr(network, net.JoinHostPort(host, port))
		if err != nil {
			return nil
		}

		return addr
	default:
		// Unsupport
	}

	return nil
}

// Net2fd is a utility helper.
//
//nolint:gosec // G103: deliberate unsafe introspection (tls.Conn unwrap + fd digging); audited, read-only use
func Net2fd(conn net.Conn) (int, error) {
	c := conn
	if c == nil {
		return -1, ErrNilConnection
	}

	if _, ok := conn.(*tls.Conn); ok {
		innerConn := reflect.Indirect(
			reflect.ValueOf(conn).Elem().FieldByName("conn"),
		)
		v := reflect.NewAt(
			innerConn.Type(),
			unsafe.Pointer(
				innerConn.UnsafeAddr(),
			),
		).Elem()
		nc, ok := reflect.TypeAssert[net.Conn](v)
		if !ok {
			return -1, fmt.Errorf("utils: Net2fd inner conn is %T, not net.Conn", v.Interface())
		}

		c = nc
	}

	fdVal := reflect.Indirect(
		reflect.ValueOf(c),
	).FieldByName("conn").FieldByName("fd")
	pfdVal := reflect.Indirect(fdVal).FieldByName("pfd")

	return int(pfdVal.FieldByName("Sysfd").Int()), nil
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
