package utils

import (
	"fmt"
	"strings"
)


// GetDNSCheckCommand returns the shell command to verify DNS configuration
func GetDNSCheckCommand(domain, expectedIP string) string {
	return fmt.Sprintf(`
		RESOLVED_IP=$(dig +short %s @8.8.8.8 | tail -n1)
		if [ -z "$RESOLVED_IP" ]; then
			echo "DNS_NOT_RESOLVED"
			exit 1
		fi
		if [ "$RESOLVED_IP" != "%s" ]; then
			echo "DNS_MISMATCH:$RESOLVED_IP"
			exit 1
		fi
		echo "DNS_OK"
	`, domain, expectedIP)
}

// ParseDNSCheckOutput parses the output from DNS check command
// Returns: (isResolved bool, resolvedIP string, err error)
func ParseDNSCheckOutput(output string) (bool, string, error) {
	output = strings.TrimSpace(output)

	if strings.Contains(output, "DNS_NOT_RESOLVED") {
		return false, "", fmt.Errorf("domain is not resolving")
	}

	if strings.Contains(output, "DNS_MISMATCH:") {
		parts := strings.Split(output, ":")
		if len(parts) >= 2 {
			resolvedIP := strings.TrimSpace(parts[1])
			return false, resolvedIP, fmt.Errorf("DNS resolving to wrong IP: %s", resolvedIP)
		}
		return false, "", fmt.Errorf("DNS mismatch detected but could not parse IP")
	}

	if strings.Contains(output, "DNS_OK") {
		return true, "", nil
	}

	return false, "", fmt.Errorf("unexpected DNS check output: %s", output)
}

// FormatDNSNotResolvedError returns a formatted error message when DNS is not resolving
func FormatDNSNotResolvedError(domain, serverIP, email string) error {
	return fmt.Errorf(`DNS is not resolving for %s

The domain must resolve to %s before SSL can be configured.

Please verify:
1. Check DNS resolution: dig %s
2. Wait for DNS propagation (can take 5-60 minutes)
3. Verify A record exists in your DNS provider
4. Once DNS is working, retry SSL setup with:
   ./selfhosted setup-ssl --app <app> --domain %s --email %s --server-ip %s`,
		domain, serverIP, domain, domain, email, serverIP)
}

// FormatDNSMismatchError returns a formatted error message when DNS resolves to wrong IP
func FormatDNSMismatchError(domain, resolvedIP, expectedIP string) error {
	return fmt.Errorf(`DNS is resolving to wrong IP address

Domain: %s
Current DNS resolution: %s
Expected IP: %s

Please update your A record to point to the correct IP address.`,
		domain, resolvedIP, expectedIP)
}
