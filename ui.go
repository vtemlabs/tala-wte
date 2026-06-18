// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package tala

import "embed"

//go:embed all:web/build
var FrontendFS embed.FS

// LicenseText is the full Tala WTE license, embedded so the app can display it in-app.
//
//go:embed LICENSE
var LicenseText string
