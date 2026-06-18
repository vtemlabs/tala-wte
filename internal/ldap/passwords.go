// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package ldap

import (
	"strconv"
	"strings"
)

// passwordTier is the strength bucket of a generated password, modeling a
// realistic corporate distribution for training.
type passwordTier int

const (
	passwordTierWeak passwordTier = iota
	passwordTierMedium
	passwordTierStrong
)

// weakCommonPasswords are breach-corpus passwords handed to the weak tier, with
// varied suffixes so the directory isn't all Password1!.
var weakCommonPasswords = []string{
	"Password1!",
	"Password123",
	"Password2024",
	"Password2025",
	"Welcome1",
	"Welcome123",
	"Welcome2024",
	"Welcome2025!",
	"ChangeMe!",
	"Letmein123!",
	"Qwerty123!",
	"Abc12345!",
	"Spring2024!",
	"Summer2024!",
	"Fall2024!",
	"Winter2024!",
	"Spring2025!",
	"Summer2025!",
	"Fall2025!",
	"Winter2025!",
	"Football1!",
	"Baseball1!",
}

// generateMixedPassword returns one password from the realistic distribution:
// 40% weak, 30% medium (semi-personalized), 30% strong random.
func generateMixedPassword(firstName, lastName, companyName string) string {
	switch rollPasswordTier() {
	case passwordTierWeak:
		// 1-in-5 weak passwords reuse the company name as "{Company}1!".
		if companyName != "" && randomInt(5) == 0 {
			compact := strings.ReplaceAll(companyName, " ", "")
			return compact + "1!"
		}
		return weakCommonPasswords[randomInt(len(weakCommonPasswords))]

	case passwordTierMedium:
		// Semi-personalized: attackable by a targeted name+year dictionary.
		switch randomInt(6) {
		case 0:
			return firstName + strconv.Itoa(2020+randomInt(6)) + "!"
		case 1:
			return lastName + strconv.Itoa(100+randomInt(900)) + "!"
		case 2:
			return firstName + "@" + lastName + strconv.Itoa(2020+randomInt(6))
		case 3:
			return strings.ToLower(firstName) + "_" + strings.ToLower(lastName) + strconv.Itoa(randomInt(100))
		case 4:
			seasons := []string{"Spring", "Summer", "Fall", "Winter"}
			return seasons[randomInt(4)] + strconv.Itoa(2020+randomInt(6)) + "!"
		default:
			return titleCase(firstName) + "!" + strconv.Itoa(1000+randomInt(9000))
		}

	default:
		// Strong tier: 12-char crypto/rand, uncrackable in an engagement window.
		return randomPassword(12)
	}
}

// titleCase uppercases the first ASCII letter of s and lowercases the rest.
func titleCase(s string) string {
	if s == "" {
		return ""
	}
	first := strings.ToUpper(s[:1])
	rest := strings.ToLower(s[1:])
	return first + rest
}

// rollPasswordTier weights the three buckets at 40/30/30.
func rollPasswordTier() passwordTier {
	r := randomInt(100)
	switch {
	case r < 40:
		return passwordTierWeak
	case r < 70:
		return passwordTierMedium
	default:
		return passwordTierStrong
	}
}
