package commons

import (
	"net/http"
	"net/netip"
	"strings"
)

func GetClientIp(r *http.Request) string {
	ip := getForwardedHost(r)
	if ip == "" {
		ip = getRemoteAddr(r)
	}
	return ip
}

func getForwardedHost(r *http.Request) string {
	hosts := r.Header.Get("X-Forwarded-For")
	if hosts == "" {
		return ""
	}

	ip := strings.Split(hosts, ",")[0]
	ip = strings.Trim(ip, " ")

	return normalizeIp(parseIp(ip))
}

func getRemoteAddr(r *http.Request) string {
	return normalizeIp(netip.MustParseAddrPort(r.RemoteAddr).Addr())
}

func parseIp(ip string) netip.Addr {
	addr, err := netip.ParseAddrPort(ip)
	if err != nil {
		return netip.MustParseAddr(ip)
	}
	return addr.Addr()
}

func normalizeIp(ip netip.Addr) string {
	return netip.AddrFrom16(ip.As16()).StringExpanded()
}
