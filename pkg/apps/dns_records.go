package apps

// DNSRecord describes a desired DNS record for an app.
// This intentionally avoids importing pkg/dns to keep packages loosely coupled.
type DNSRecord struct {
	Type    string
	Name    string
	Content string
	TTL     int
	Proxied *bool
}

// DNSRecordProvider is an optional interface apps can implement to request custom DNS records.
// If not implemented or it returns an empty slice, the deploy flow falls back to a single record for opts.Domain.
type DNSRecordProvider interface {
	DNSRecords(domain string, serverIP string) []DNSRecord
}
