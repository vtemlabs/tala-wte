// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package main

import (
	"log"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/vtemlabs/tala-wte/internal/portal"
	"github.com/vtemlabs/tala-wte/internal/sim"
)

// PocketBase TextField caps at 5000 chars when Max is unset; portal HTML routinely exceeds that, so these opt into a large explicit limit.
const (
	portalHTMLMax     = 8000000
	submissionDataMax = 200000
)

// portalCategories is the allowed-value list for the portals.category select field; a category not in it is rejected on save.
var portalCategories = []string{
	"coffee", "restaurant", "retail", "hotel", "corporate", "airport",
	"inflight", "transit", "isp", "telecom", "education", "healthcare",
	"library", "event", "cruise", "fitness", "automotive", "social",
	"generic", "custom",
}

// bootstrapCollections ensures required PocketBase collections exist.
func bootstrapCollections(app *pocketbase.PocketBase) {
	collections := []struct {
		name   string
		fields []core.Field
	}{
		{
			name: "portals",
			fields: []core.Field{
				&core.TextField{Name: "name", Required: true},
				&core.TextField{Name: "html", Required: true, Max: portalHTMLMax},
				&core.SelectField{Name: "type", Values: []string{"builtin", "custom"}},
				&core.TextField{Name: "slug"},
				&core.SelectField{Name: "category", Values: portalCategories},
				&core.TextField{Name: "description"},
			},
		},
		{
			name: "networks",
			fields: []core.Field{
				&core.TextField{Name: "ssid", Required: true},
				&core.SelectField{Name: "protocol", Required: true, Values: []string{
					"open", "wep", "wpa", "wpa2", "wps", "wpa3", "wpa3_transition",
					"wpa2_enterprise", "wpa3_enterprise",
				}},
				&core.SelectField{Name: "band", Values: []string{"2.4", "5", "6"}},
				&core.NumberField{Name: "channel"},
				&core.TextField{Name: "passphrase"},
				&core.TextField{Name: "identity"},     // 802.1X EAP identity (enterprise)
				&core.TextField{Name: "eap_password"}, // 802.1X EAP password (enterprise)
				&core.SelectField{Name: "status", Values: []string{"stopped", "starting", "running", "error"}},
				&core.TextField{Name: "interface"},
				&core.TextField{Name: "config_json"},
				&core.RelationField{Name: "portal_id", CollectionId: "portals"},
				&core.BoolField{Name: "portal_enabled"},
				&core.TextField{Name: "portal_html", Max: portalHTMLMax},
				&core.BoolField{Name: "client_isolation"},
				&core.BoolField{Name: "internet_passthrough"},
				&core.BoolField{Name: "portal_auth"},
				&core.BoolField{Name: "hidden"},
				&core.TextField{Name: "subnet"},
			},
		},
		{
			name: "settings",
			fields: []core.Field{
				&core.TextField{Name: "key", Required: true},
				&core.TextField{Name: "value"},
			},
		},
		{
			name: "certificates",
			fields: []core.Field{
				&core.TextField{Name: "name", Required: true},
				&core.SelectField{Name: "type", Values: []string{"ca", "server", "client"}},
				&core.FileField{Name: "cert_file"},
				&core.FileField{Name: "key_file"},
				&core.RelationField{Name: "network_id", CollectionId: "networks", CascadeDelete: true},
				&core.DateField{Name: "expires_at"},
			},
		},
		{
			name: "captures",
			fields: []core.Field{
				&core.RelationField{Name: "network_id", CollectionId: "networks", CascadeDelete: true},
				&core.SelectField{Name: "layer", Values: []string{"wireless", "network"}},
				&core.TextField{Name: "interface"},
				&core.TextField{Name: "filter"},
				&core.FileField{Name: "file"},
				&core.NumberField{Name: "packet_count"},
				&core.SelectField{Name: "status", Values: []string{"idle", "running", "stopped", "error"}},
				&core.DateField{Name: "started_at"},
				&core.DateField{Name: "stopped_at"},
			},
		},
		{
			name: "traffic_datasets",
			fields: []core.Field{
				&core.TextField{Name: "name", Required: true},
				&core.TextField{Name: "slug"},
				&core.TextField{Name: "description"},
				&core.TextField{Name: "urls", Max: 20000},
				&core.TextField{Name: "domains", Max: 20000},
				&core.TextField{Name: "ips", Max: 20000},
				&core.SelectField{Name: "type", Values: []string{"builtin", "custom"}},
			},
		},
		{
			name: "clients",
			fields: []core.Field{
				&core.RelationField{Name: "network_id", CollectionId: "networks", CascadeDelete: true},
				&core.TextField{Name: "mac"},
				&core.TextField{Name: "ip"},
				&core.TextField{Name: "hostname"},
				&core.NumberField{Name: "signal"},
				&core.DateField{Name: "connected_at"},
				&core.DateField{Name: "disconnected_at"},
			},
		},
		{
			name: "portal_submissions",
			fields: []core.Field{
				&core.RelationField{Name: "network_id", CollectionId: "networks", CascadeDelete: true},
				&core.TextField{Name: "network_ssid"},
				&core.TextField{Name: "mac"},
				&core.TextField{Name: "ip"},
				&core.TextField{Name: "user_agent"},
				&core.TextField{Name: "data_json", Max: submissionDataMax},
				&core.AutodateField{Name: "created", OnCreate: true},
				&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
			},
		},
		{
			name: "radius_config",
			fields: []core.Field{
				&core.RelationField{Name: "network_id", CollectionId: "networks", CascadeDelete: true},
				&core.SelectField{Name: "eap_type", Values: []string{"peap", "tls", "ttls", "fast"}},
				&core.SelectField{Name: "inner_auth", Values: []string{"mschapv2", "pap", "chap", "gtc"}},
				&core.RelationField{Name: "ca_cert_id", CollectionId: "certificates"},
				&core.RelationField{Name: "server_cert_id", CollectionId: "certificates"},
			},
		},
		{
			// Saved client connection profiles. In client mode the operator uploads
			// configs exported from APs; they persist here so any one can be reused.
			name: "client_configs",
			fields: []core.Field{
				&core.TextField{Name: "ssid", Required: true},
				&core.TextField{Name: "protocol"},
				&core.TextField{Name: "passphrase"},
				&core.TextField{Name: "band"},
				&core.NumberField{Name: "channel"},
				&core.BoolField{Name: "hidden"},
				&core.TextField{Name: "identity"},
				&core.TextField{Name: "eap_password"},
				&core.BoolField{Name: "portal_enabled"},
				&core.TextField{Name: "portal_username"},
				&core.TextField{Name: "portal_password"},
				&core.AutodateField{Name: "created", OnCreate: true},
			},
		},
		{
			// Den members: client instances this server (the den leader) drives. The
			// leader reaches each by address using its agent_key and tracks which
			// network it is currently assigned to.
			name: "den_members",
			fields: []core.Field{
				&core.TextField{Name: "name", Required: true},
				&core.TextField{Name: "address", Required: true},
				&core.TextField{Name: "agent_key", Required: true},
				&core.TextField{Name: "cert_fingerprint"},
				&core.TextField{Name: "network_id"},
				&core.AutodateField{Name: "created", OnCreate: true},
			},
		},
	}

	for _, c := range collections {
		existing, _ := app.FindCollectionByNameOrId(c.name)
		if existing != nil {
			continue
		}
		log.Printf("bootstrapping collection: %s", c.name)

		col := core.NewBaseCollection(c.name)

		if c.name == "settings" {
			col.AddIndex("idx_settings_key", true, "key", "")
		}

		// Leave API rules nil (locked) so only superusers reach these collections;
		// PocketBase treats "" as PUBLIC. See reconcileCollectionRules.

		for _, f := range c.fields {
			// Resolve relation CollectionId from name to the actual internal ID.
			if relField, ok := f.(*core.RelationField); ok {
				if dependent, err := app.FindCollectionByNameOrId(relField.CollectionId); err == nil {
					relField.CollectionId = dependent.Id
				}
			}
			col.Fields.Add(f)
		}

		if err := app.Save(col); err != nil {
			log.Printf("[bootstrap] failed to create collection %s: %v", c.name, err)
		}
	}

	// The create-only loop above skips existing collections, so these reconcilers
	// migrate databases created by earlier versions.
	reconcileNetworkProtocols(app)
	reconcileCollectionRules(app)
	reconcileDenMemberSchema(app)
	reconcileNetworkSchema(app)
	removeDefaultUsersCollection(app)
	reconcilePortalFields(app)
	reconcileNetworkPortalAuth(app)
	reconcileNetworkHidden(app)
	reconcileNetworkSubnet(app)
	reconcileSubmissionTimestamps(app)
	reconcilePortalHTMLLimits(app)

	// MUST run before seeding, or templates in the new categories are rejected.
	reconcilePortalCategories(app)

	seedPortalTemplates(app)
	seedTrafficDatasets(app)

	// Kill rogue hostapd/dnsmasq/socat and purge wte-* namespaces.
	sim.NuclearTeardown("boot")

	resetNetworkStatuses(app)
}

// reconcileNetworkProtocols keeps the networks.protocol select options in sync on databases created before a protocol was added.
func reconcileNetworkProtocols(app *pocketbase.PocketBase) {
	nets, err := app.FindCollectionByNameOrId("networks")
	if err != nil || nets == nil {
		return
	}
	field, ok := nets.Fields.GetByName("protocol").(*core.SelectField)
	if !ok {
		return
	}
	want := []string{"open", "wep", "wpa", "wpa2", "wps", "wpa3", "wpa3_transition", "wpa2_enterprise", "wpa3_enterprise"}
	if strings.Join(field.Values, ",") == strings.Join(want, ",") {
		return
	}
	field.Values = want
	if err := app.Save(nets); err != nil {
		log.Printf("[bootstrap] failed to update networks.protocol values: %v", err)
	} else {
		log.Printf("[bootstrap] reconciled networks.protocol values (now includes wep)")
	}
}

// reconcilePortalCategories keeps the portals.category select options in sync on databases created before a category was added.
func reconcilePortalCategories(app *pocketbase.PocketBase) {
	col, err := app.FindCollectionByNameOrId("portals")
	if err != nil || col == nil {
		return
	}
	field, ok := col.Fields.GetByName("category").(*core.SelectField)
	if !ok {
		return
	}
	if strings.Join(field.Values, ",") == strings.Join(portalCategories, ",") {
		return
	}
	field.Values = portalCategories
	if err := app.Save(col); err != nil {
		log.Printf("[bootstrap] failed to update portals.category values: %v", err)
	} else {
		log.Printf("[bootstrap] reconciled portals.category values (%d categories)", len(portalCategories))
	}
}

// reconcilePortalFields adds the gallery metadata fields (slug, category, description) to a portals collection that predates them.
func reconcilePortalFields(app *pocketbase.PocketBase) {
	col, err := app.FindCollectionByNameOrId("portals")
	if err != nil || col == nil {
		return
	}
	if col.Fields.GetByName("slug") != nil {
		return
	}
	col.Fields.Add(&core.TextField{Name: "slug"})
	col.Fields.Add(&core.SelectField{Name: "category", Values: portalCategories})
	col.Fields.Add(&core.TextField{Name: "description"})
	if err := app.Save(col); err != nil {
		log.Printf("[bootstrap] failed to add portal gallery fields: %v", err)
	} else {
		log.Printf("[bootstrap] added portal gallery fields (slug, category, description)")
	}
}

// reconcileNetworkPortalAuth adds the networks.portal_auth bool field to databases that predate it.
func reconcileNetworkPortalAuth(app *pocketbase.PocketBase) {
	col, err := app.FindCollectionByNameOrId("networks")
	if err != nil || col == nil {
		return
	}
	if col.Fields.GetByName("portal_auth") != nil {
		return
	}
	col.Fields.Add(&core.BoolField{Name: "portal_auth"})
	if err := app.Save(col); err != nil {
		log.Printf("[bootstrap] failed to add networks.portal_auth field: %v", err)
	} else {
		log.Printf("[bootstrap] added networks.portal_auth field")
	}
}

// reconcileNetworkHidden adds the networks.hidden bool field to databases that predate it.
func reconcileNetworkHidden(app *pocketbase.PocketBase) {
	col, err := app.FindCollectionByNameOrId("networks")
	if err != nil || col == nil {
		return
	}
	if col.Fields.GetByName("hidden") != nil {
		return
	}
	col.Fields.Add(&core.BoolField{Name: "hidden"})
	if err := app.Save(col); err != nil {
		log.Printf("[bootstrap] failed to add networks.hidden field: %v", err)
	} else {
		log.Printf("[bootstrap] added networks.hidden field")
	}
}

// reconcileNetworkSubnet adds the networks.subnet text field to databases that predate it.
func reconcileNetworkSubnet(app *pocketbase.PocketBase) {
	col, err := app.FindCollectionByNameOrId("networks")
	if err != nil || col == nil {
		return
	}
	if col.Fields.GetByName("subnet") != nil {
		return
	}
	col.Fields.Add(&core.TextField{Name: "subnet"})
	if err := app.Save(col); err != nil {
		log.Printf("[bootstrap] failed to add networks.subnet field: %v", err)
	} else {
		log.Printf("[bootstrap] added networks.subnet field")
	}
}

// reconcileSubmissionTimestamps adds created/updated autodate fields to a portal_submissions collection that predates them.
func reconcileSubmissionTimestamps(app *pocketbase.PocketBase) {
	col, err := app.FindCollectionByNameOrId("portal_submissions")
	if err != nil || col == nil {
		return
	}
	if col.Fields.GetByName("created") != nil {
		return
	}
	col.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	col.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
	if err := app.Save(col); err != nil {
		log.Printf("[bootstrap] failed to add portal_submissions timestamps: %v", err)
	} else {
		log.Printf("[bootstrap] added portal_submissions created/updated fields")
	}
}

// reconcilePortalHTMLLimits raises the max length on HTML-bearing text fields of collections that already exist.
func reconcilePortalHTMLLimits(app *pocketbase.PocketBase) {
	bump := func(collection, field string, max int) {
		col, err := app.FindCollectionByNameOrId(collection)
		if err != nil || col == nil {
			return
		}
		tf, ok := col.Fields.GetByName(field).(*core.TextField)
		if !ok || tf.Max >= max {
			return
		}
		tf.Max = max
		if err := app.Save(col); err != nil {
			log.Printf("[bootstrap] failed to raise %s.%s max: %v", collection, field, err)
		} else {
			log.Printf("[bootstrap] raised %s.%s max length to %d", collection, field, max)
		}
	}
	bump("portals", "html", portalHTMLMax)
	bump("networks", "portal_html", portalHTMLMax)
	bump("portal_submissions", "data_json", submissionDataMax)
}

// seedPortalTemplates upserts the embedded built-in portal templates into the
// portals collection, keyed by slug: it recreates any deleted built-in and
// re-syncs a changed one back to the embedded source. User clones (a different
// slug, type=custom) are never touched. Returns how many were created/re-synced.
func seedPortalTemplates(app *pocketbase.PocketBase) (seeded, resynced int) {
	col, err := app.FindCollectionByNameOrId("portals")
	if err != nil || col == nil {
		return 0, 0
	}
	templates, err := portal.Catalog()
	if err != nil {
		log.Printf("[bootstrap] failed to load embedded portal templates: %v", err)
		return 0, 0
	}
	for _, t := range templates {
		existing, _ := app.FindFirstRecordByFilter("portals", "slug = {:slug}", map[string]any{"slug": t.Slug})
		if existing != nil {
			// Keep managed built-ins in sync with the embedded source; user clones carry a different slug and are never touched.
			if existing.GetString("type") == "builtin" && existing.GetString("html") != t.HTML {
				existing.Set("html", t.HTML)
				existing.Set("name", t.Name)
				existing.Set("category", t.Category)
				existing.Set("description", t.Description)
				if err := app.Save(existing); err != nil {
					log.Printf("[bootstrap] failed to update built-in portal %s: %v", t.Slug, err)
				} else {
					resynced++
				}
			}
			continue
		}
		rec := core.NewRecord(col)
		rec.Set("name", t.Name)
		rec.Set("html", t.HTML)
		rec.Set("type", "builtin")
		rec.Set("slug", t.Slug)
		rec.Set("category", t.Category)
		rec.Set("description", t.Description)
		if err := app.Save(rec); err != nil {
			log.Printf("[bootstrap] failed to seed portal template %s: %v", t.Slug, err)
			continue
		}
		seeded++
	}
	if seeded > 0 {
		log.Printf("[bootstrap] seeded %d built-in portal templates", seeded)
	}
	if resynced > 0 {
		log.Printf("[bootstrap] re-synced %d built-in portal templates to embedded source", resynced)
	}
	return seeded, resynced
}

// seedTrafficDatasets upserts the built-in traffic-target datasets (created once,
// keyed by slug; user edits and custom datasets are never touched). They default
// to public endpoints that are designed for automated/connectivity probes.
func seedTrafficDatasets(app *pocketbase.PocketBase) {
	col, err := app.FindCollectionByNameOrId("traffic_datasets")
	if err != nil || col == nil {
		return
	}
	builtins := []struct{ Slug, Name, Description, URLs, Domains, IPs string }{
		{
			"connectivity-checks", "Connectivity & captive checks",
			"OS connectivity and captive-portal endpoints, designed for automated probes.",
			"http://captive.apple.com/\nhttp://connectivitycheck.gstatic.com/generate_204\nhttp://detectportal.firefox.com/canonical.html\nhttp://neverssl.com/",
			"captive.apple.com\nconnectivitycheck.gstatic.com\ndetectportal.firefox.com",
			"1.1.1.1\n8.8.8.8",
		},
		{
			"general-browsing", "General browsing (safe)",
			"Public endpoints intended for testing automated HTTP traffic.",
			"http://example.com/\nhttp://example.org/\nhttps://httpbin.org/get\nhttp://neverssl.com/",
			"example.com\nexample.org\nhttpbin.org",
			"",
		},
		{
			"local-intranet", "Local / intranet",
			"Traffic kept on the lab network: the gateway and an intranet host.",
			"http://10.0.0.1/\nhttp://intranet.local/",
			"intranet.local",
			"10.0.0.1",
		},
		{
			"dns-chatter", "DNS chatter",
			"A domain set for steady DNS resolution traffic.",
			"",
			"example.com\nexample.org\nneverssl.com\nhttpbin.org\ncaptive.apple.com\nconnectivitycheck.gstatic.com",
			"",
		},
	}
	seeded := 0
	for _, d := range builtins {
		existing, _ := app.FindFirstRecordByFilter("traffic_datasets", "slug = {:slug}", map[string]any{"slug": d.Slug})
		if existing != nil {
			continue
		}
		rec := core.NewRecord(col)
		rec.Set("name", d.Name)
		rec.Set("slug", d.Slug)
		rec.Set("description", d.Description)
		rec.Set("urls", d.URLs)
		rec.Set("domains", d.Domains)
		rec.Set("ips", d.IPs)
		rec.Set("type", "builtin")
		if err := app.Save(rec); err != nil {
			log.Printf("[bootstrap] failed to seed traffic dataset %s: %v", d.Slug, err)
			continue
		}
		seeded++
	}
	if seeded > 0 {
		log.Printf("[bootstrap] seeded %d built-in traffic datasets", seeded)
	}
}

// reconcileCollectionRules locks the managed collections to superusers only (nil rules); PocketBase treats "" as PUBLIC.
func reconcileCollectionRules(app *pocketbase.PocketBase) {
	managed := []string{"portals", "networks", "settings", "certificates", "captures", "clients", "radius_config", "portal_submissions", "den_members", "client_configs", "traffic_datasets"}
	for _, name := range managed {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil || col == nil {
			continue
		}
		if col.ListRule == nil && col.ViewRule == nil && col.CreateRule == nil &&
			col.UpdateRule == nil && col.DeleteRule == nil {
			continue
		}
		col.ListRule = nil
		col.ViewRule = nil
		col.CreateRule = nil
		col.UpdateRule = nil
		col.DeleteRule = nil
		if err := app.Save(col); err != nil {
			log.Printf("[bootstrap] failed to lock collection %s: %v", name, err)
		} else {
			log.Printf("[bootstrap] locked collection %s to superusers only", name)
		}
	}
}

// reconcileDenMemberSchema backfills the cert_fingerprint field on a den_members
// collection created before certificate pinning existed. New installs get it from
// the collection definition; this upgrades existing ones so the leader can pin
// each member's self-signed certificate on first contact.
func reconcileDenMemberSchema(app *pocketbase.PocketBase) {
	col, err := app.FindCollectionByNameOrId("den_members")
	if err != nil || col == nil {
		return
	}
	if col.Fields.GetByName("cert_fingerprint") != nil {
		return
	}
	col.Fields.Add(&core.TextField{Name: "cert_fingerprint"})
	if err := app.Save(col); err != nil {
		log.Printf("[bootstrap] failed to add cert_fingerprint to den_members: %v", err)
	}
}

// reconcileNetworkSchema backfills the identity + eap_password fields on a networks
// collection created before enterprise EAP credentials were stored per network. New
// installs get them from the collection definition; this upgrades existing ones so a
// den deploy (and the config export) can hand 802.1X clients the identity/password
// they present to RADIUS.
func reconcileNetworkSchema(app *pocketbase.PocketBase) {
	col, err := app.FindCollectionByNameOrId("networks")
	if err != nil || col == nil {
		return
	}
	changed := false
	if col.Fields.GetByName("identity") == nil {
		col.Fields.Add(&core.TextField{Name: "identity"})
		changed = true
	}
	if col.Fields.GetByName("eap_password") == nil {
		col.Fields.Add(&core.TextField{Name: "eap_password"})
		changed = true
	}
	if changed {
		if err := app.Save(col); err != nil {
			log.Printf("[bootstrap] failed to add EAP fields to networks: %v", err)
		}
	}
}

// removeDefaultUsersCollection deletes (or, failing that, locks) PocketBase's default "users" auth collection, which ships with public self-registration.
func removeDefaultUsersCollection(app *pocketbase.PocketBase) {
	col, err := app.FindCollectionByNameOrId("users")
	if err != nil || col == nil {
		return
	}
	if delErr := app.Delete(col); delErr == nil {
		log.Printf("[bootstrap] removed default public 'users' collection")
		return
	}
	col.ListRule = nil
	col.ViewRule = nil
	col.CreateRule = nil
	col.UpdateRule = nil
	col.DeleteRule = nil
	if saveErr := app.Save(col); saveErr != nil {
		log.Printf("[bootstrap] failed to lock 'users' collection: %v", saveErr)
	} else {
		log.Printf("[bootstrap] locked default 'users' collection (no public signup)")
	}
}

func resetNetworkStatuses(app *pocketbase.PocketBase) {
	records, err := app.FindRecordsByFilter("networks", "status != 'stopped'", "", 0, 0)
	if err == nil {
		for _, r := range records {
			// A network that was "running" before this boot should be auto-restarted
			// once the system settles; record it before we reset the live status.
			if r.GetString("status") == "running" {
				pendingAutostart = append(pendingAutostart, r.Id)
			}
			r.Set("status", "stopped")
			if err := app.Save(r); err != nil {
				log.Printf("[bootstrap] failed to reset network status for %s: %v", r.Id, err)
			}
		}
		if len(records) > 0 {
			log.Printf("Reset %d stale network statuses to stopped on boot", len(records))
		}
	}
}
