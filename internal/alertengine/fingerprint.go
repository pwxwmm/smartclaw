package alertengine

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// Labels excluded from fingerprinting because they vary per instance.
var excludedLabels = map[string]bool{
	"instance":     true,
	"pod":          true,
	"container":    true,
	"hostname":     true,
	"pod_name":     true,
	"container_id": true,
	"job":          true,
	"endpoint":     true,
}

// NormalizeLabels strips instance-specific labels that should not
// contribute to fingerprinting.
func NormalizeLabels(labels map[string]string) map[string]string {
	normalized := make(map[string]string, len(labels))
	for k, v := range labels {
		if !excludedLabels[k] {
			normalized[k] = v
		}
	}
	return normalized
}

// FingerprintAlert computes a SHA-256 fingerprint for an alert based on
// source, name, service, and sorted normalized label pairs.
func FingerprintAlert(alert *Alert) string {
	normalized := NormalizeLabels(alert.Labels)

	keys := make([]string, 0, len(normalized))
	for k := range normalized {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(alert.Source)
	b.WriteByte(0)
	b.WriteString(alert.Name)
	b.WriteByte(0)
	b.WriteString(alert.Service)
	b.WriteByte(0)

	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(normalized[k])
		b.WriteByte(0)
	}

	hash := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("%x", hash)
}
