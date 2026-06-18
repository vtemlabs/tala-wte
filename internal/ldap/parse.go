// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package ldap

import (
	"encoding/base64"
	"strings"
)

// parseLDIF parses ldapsearch -LLL output into attribute maps, decoding RFC 2849
// base64 values (`attr:: <base64>`) that slapd uses for non-printable or binary
// attributes like userPassword.
func parseLDIF(output string) []map[string]string {
	var entries []map[string]string
	var current map[string]string

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			if current != nil {
				entries = append(entries, current)
				current = nil
			}
			continue
		}
		if strings.HasPrefix(line, " ") {
			continue // LDIF continuation line
		}

		// Check `:: ` (base64) before `: `, since the latter is a substring.
		var key, val string
		var b64 bool
		if idx := strings.Index(line, ":: "); idx >= 0 {
			key = strings.ToLower(strings.TrimSpace(line[:idx]))
			val = strings.TrimSpace(line[idx+3:])
			b64 = true
		} else if idx := strings.Index(line, ": "); idx >= 0 {
			key = strings.ToLower(strings.TrimSpace(line[:idx]))
			val = strings.TrimSpace(line[idx+2:])
		} else {
			continue
		}

		if b64 {
			if decoded, err := base64.StdEncoding.DecodeString(val); err == nil {
				val = string(decoded)
			}
		}

		if key == "dn" {
			if current != nil {
				entries = append(entries, current)
			}
			current = map[string]string{"dn": val}
		} else if current != nil {
			if existing, ok := current[key]; ok {
				current[key] = existing + "\n" + val
			} else {
				current[key] = val
			}
		}
	}

	if current != nil {
		entries = append(entries, current)
	}
	return entries
}
