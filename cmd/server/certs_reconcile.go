// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package main

import (
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/vtemlabs/tala-wte/internal/certs"
)

// reconcileCerts mirrors the on-disk PKI into the certificates collection the UI
// lists. The cert handlers only write the filesystem, so without this the
// Certificates page shows "No CA" and an empty table even when a working CA and
// issued certs exist on disk (and enterprise networks use them).
func reconcileCerts(app *pocketbase.PocketBase) {
	coll, err := app.FindCollectionByNameOrId("certificates")
	if err != nil {
		return
	}
	entries, err := os.ReadDir(certs.CADir())
	if err != nil {
		return // no PKI yet
	}
	existing, _ := app.FindAllRecords("certificates")
	byName := map[string]*core.Record{}
	for _, r := range existing {
		byName[r.GetString("name")] = r
	}

	onDisk := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".crt") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".crt")
		data, err := os.ReadFile(filepath.Join(certs.CADir(), e.Name()))
		if err != nil {
			continue
		}
		block, _ := pem.Decode(data)
		if block == nil {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		onDisk[base] = true

		ctype := "client"
		switch {
		case cert.IsCA:
			ctype = "ca"
		case hasExtKeyUsage(cert, x509.ExtKeyUsageServerAuth):
			ctype = "server"
		}

		rec := byName[base]
		if rec == nil {
			rec = core.NewRecord(coll)
			rec.Set("name", base)
		}
		rec.Set("type", ctype)
		if dt, err := types.ParseDateTime(cert.NotAfter); err == nil {
			rec.Set("expires_at", dt)
		}
		_ = app.Save(rec)
	}

	// Drop records whose backing cert is gone from disk.
	for name, rec := range byName {
		if !onDisk[name] {
			_ = app.Delete(rec)
		}
	}
}

func hasExtKeyUsage(c *x509.Certificate, want x509.ExtKeyUsage) bool {
	for _, eku := range c.ExtKeyUsage {
		if eku == want {
			return true
		}
	}
	return false
}

// certReconcileAfter runs h, then reconciles the certificates collection so a
// newly created CA or cert appears on the page immediately.
func certReconcileAfter(app *pocketbase.PocketBase, h func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r)
		reconcileCerts(app)
	}
}
