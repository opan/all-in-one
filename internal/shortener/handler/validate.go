package handler

import (
	"fmt"
	"net"
	"net/url"

	"github.com/all-in-one/internal/config"
)

func validateURL(rawURL string, cfg config.ShortenerURLConfig) error {
	if len(rawURL) == 0 {
		return fmt.Errorf("target_url is required")
	}
	if len(rawURL) > cfg.MaxLength {
		return fmt.Errorf("target_url exceeds maximum length of %d", cfg.MaxLength)
	}

	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("target_url is not a valid URL")
	}

	allowed := cfg.AllowedSchemes
	if len(allowed) == 0 {
		allowed = []string{"http", "https"}
	}
	schemeOK := false
	for _, s := range allowed {
		if parsed.Scheme == s {
			schemeOK = true
			break
		}
	}
	if !schemeOK {
		return fmt.Errorf("URL scheme %q is not allowed", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("target_url must be an absolute URL with a host")
	}

	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("target_url must not point to a private or loopback address")
		}
	}

	for _, blocked := range cfg.BlockedHosts {
		if host == blocked {
			return fmt.Errorf("target_url host is not allowed")
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return ip.IsLoopback() || ip.IsUnspecified()
}
