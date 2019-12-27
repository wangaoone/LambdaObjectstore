package global

import (
	"errors"
	"net"
)

var (
	ErrPrivateIPNotFound = errors.New("Can not identify private ip.")
	ErrPublicIPNotFound  = errors.New("Can not identify public ip.")
)

func GetIP(vpc bool) (string, error) {
	if vpc == true {
		return GetPrivateIp()
	} else {
		return GetPublicIp()
	}
}
func GetPublicIp() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && IsPublicIP(ipnet.IP) {
			return ipnet.IP.String(), nil
		}
	}

	return "", ErrPublicIPNotFound
}

func GetPrivateIp() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && IsPrivateIp(ipnet.IP) {
			return ipnet.IP.String(), nil
		}
	}

	return "", ErrPrivateIPNotFound
}

func IsPrivateIp(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 {
			return true
		} else if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		} else if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
	}
	return false
}

func IsPublicIP(IP net.IP) bool {
	if IP.IsLoopback() || IP.IsLinkLocalMulticast() || IP.IsLinkLocalUnicast() {
		return false
	}
	if ip4 := IP.To4(); ip4 != nil {
		switch {
		case ip4[0] == 10:
			return false
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return false
		case ip4[0] == 192 && ip4[1] == 168:
			return false
		default:
			return true
		}
	}
	return false
}
