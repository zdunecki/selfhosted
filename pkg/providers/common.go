package providers

import (
	"strings"
)

// sanitizeHostname converts an arbitrary string into a DNS-safe hostname label.
// It keeps only [a-z0-9-], collapses invalid runs into '-', trims, and limits to 63 chars.
func sanitizeHostname(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	// Keep simple: a-z0-9 and '-' only.
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		isAZ := r >= 'a' && r <= 'z'
		is09 := r >= '0' && r <= '9'
		if isAZ || is09 {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 63 {
		out = out[:63]
		out = strings.Trim(out, "-")
	}
	return out
}

// pickBestSizeForSpecs returns the "best" Size that satisfies the requested specs.
//
// Matching:
// - CPUs and MemoryMB are required comparisons
// - DiskGB is compared only if specs.DiskGB > 0
//
// Ranking:
// - Prefer sizes with a known (non-zero) monthly price, if any exist
// - Among priced: cheapest monthly price
// - Otherwise: smallest resources (vcpus, memory, disk)
func pickBestSizeForSpecs(sizes []Size, specs Specs) (*Size, bool) {
	var best *Size

	hasAnyPriced := false
	for i := range sizes {
		if sizes[i].PriceMonthly > 0 {
			hasAnyPriced = true
			break
		}
	}

	for i := range sizes {
		s := &sizes[i]
		if s.VCPUs < specs.CPUs || s.MemoryMB < specs.MemoryMB {
			continue
		}
		if specs.DiskGB > 0 && s.DiskGB < specs.DiskGB {
			continue
		}

		if best == nil {
			best = s
			continue
		}

		// If we have at least one priced size, prefer priced sizes.
		if hasAnyPriced {
			bestPriced := best.PriceMonthly > 0
			sPriced := s.PriceMonthly > 0
			if sPriced && !bestPriced {
				best = s
				continue
			}
			if sPriced && bestPriced && s.PriceMonthly < best.PriceMonthly {
				best = s
				continue
			}
			continue
		}

		// Otherwise, prefer the smallest resources.
		if s.VCPUs < best.VCPUs ||
			(s.VCPUs == best.VCPUs && s.MemoryMB < best.MemoryMB) ||
			(s.VCPUs == best.VCPUs && s.MemoryMB == best.MemoryMB && s.DiskGB < best.DiskGB) {
			best = s
			continue
		}
	}

	return best, best != nil
}
