// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package portal

import (
	"embed"
	"fmt"
	"sort"
)

//go:embed templates/*.html
var templateFS embed.FS

// Template describes a built-in captive portal template embedded in the binary.
type Template struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	HTML        string `json:"html"`
	file        string
}

// templateAuthType maps each built-in template slug to its captive-portal auth
// type, derived from the template's actual form fields. Built-ins cannot be edited
// in the UI, so this is their authoritative auth type.
var templateAuthType = map[string]AuthType{
	"airport-summit":       AuthEmail,
	"automotive-tripstop":  AuthEmail,
	"automotive-voltway":   AuthEmail,
	"coffee-driftwood":     AuthInfoForm,
	"coffee-sunrise":       AuthClickThrough,
	"corp-aruba":           AuthInfoForm,
	"corp-azuread":         AuthUserPassword,
	"corp-fortinet":        AuthUserPassword,
	"corp-meraki":          AuthClickThrough,
	"corp-selfreg":         AuthInfoForm,
	"corp-unifi":           AuthVoucher,
	"cruise-azure":         AuthHotel,
	"cruise-ferry":         AuthEmail,
	"education-k12":        AuthInfoForm,
	"education-university": AuthUserPassword,
	"event-arena":          AuthEmail,
	"event-expo":           AuthVoucher,
	"fitness-pulse":        AuthMembership,
	"generic-tos":          AuthClickThrough,
	"healthcare-cedar":     AuthEmail,
	"hotel-aria":           AuthHotel,
	"hotel-collective":     AuthEmail,
	"hotel-resteasy":       AuthHotel,
	"hotel-wexley":         AuthHotel,
	"inflight-altius":      AuthUserPassword,
	"isp-beacon":           AuthUserPassword,
	"isp-fibernet":         AuthEmailPassword,
	"library-riverside":    AuthInfoForm,
	"restaurant-harvest":   AuthInfoForm,
	"restaurant-quickbite": AuthClickThrough,
	"retail-bigbox":        AuthEmail,
	"retail-marketplace":   AuthEmail,
	"social-cityconnect":   AuthEmail,
	"telecom-optimax":      AuthUserPassword,
	"transit-metro":        AuthEmail,
	"transit-rail":         AuthInfoForm,
}

// AuthTypeForSlug returns the auth type for a built-in template slug, defaulting
// to click-through for any not explicitly mapped.
func AuthTypeForSlug(slug string) AuthType {
	if t, ok := templateAuthType[slug]; ok {
		return t
	}
	return AuthClickThrough
}

// catalog is the ordered manifest of built-in templates, each mapping to a file under templates/.
var catalog = []Template{
	{
		Slug:        "coffee-driftwood",
		Name:        "Driftwood Coffee Co.",
		Category:    "coffee",
		Description: "Coffee-shop email gate with marketing opt-in (Starbucks / Google WiFi style). Captures first name, last name, email, postal code.",
		file:        "templates/coffee-driftwood.html",
	},
	{
		Slug:        "coffee-sunrise",
		Name:        "Sunrise Bakehouse",
		Category:    "coffee",
		Description: "Friendly bakery-cafe one-tap terms gate with a 13+ age acknowledgement (Panera / Dunkin style).",
		file:        "templates/coffee-sunrise.html",
	},
	{
		Slug:        "hotel-wexley",
		Name:        "The Wexley Hotels & Resorts",
		Category:    "hotel",
		Description: "Luxury tiered hospitality portal: choose free / high-speed / enhanced, pick days, then verify by room number + last name (Marriott / GuestTek style).",
		file:        "templates/hotel-wexley.html",
	},
	{
		Slug:        "hotel-aria",
		Name:        "Aria Hotels",
		Category:    "hotel",
		Description: "Dual-login hotel portal: connect as a guest (last name + room) or sign in to the loyalty program for premium WiFi (Hilton Honors style).",
		file:        "templates/hotel-aria.html",
	},
	{
		Slug:        "corp-unifi",
		Name:        "Meridian Technologies Guest",
		Category:    "corporate",
		Description: "Corporate guest Wi-Fi click-through with company logo, acceptable-use checkbox, and an optional voucher code (UniFi / Ubiquiti style).",
		file:        "templates/corp-unifi.html",
	},
	{
		Slug:        "corp-meraki",
		Name:        "Continuum Logistics Guest",
		Category:    "corporate",
		Description: "Minimal corporate splash: welcome message and a single 'Continue to the Internet' button with implied terms acceptance (Cisco Meraki style).",
		file:        "templates/corp-meraki.html",
	},
	{
		Slug:        "corp-selfreg",
		Name:        "Stratford Group Visitor Registration",
		Category:    "corporate",
		Description: "Visitor self-registration capturing name, email, company, phone, and sponsor/host email with sponsor approval (Cisco ISE / Aruba ClearPass style).",
		file:        "templates/corp-selfreg.html",
	},
	{
		Slug:        "corp-fortinet",
		Name:        "Halcyon Manufacturing Login",
		Category:    "corporate",
		Description: "Plain firewall-style captive portal requiring a username and password to authenticate to the network (FortiGate style).",
		file:        "templates/corp-fortinet.html",
	},
	{
		Slug:        "airport-summit",
		Name:        "Summit International Airport",
		Category:    "airport",
		Description: "Airport free-WiFi splash with free vs premium plan cards, email capture, terms gate, and a sponsor slot.",
		file:        "templates/airport-summit.html",
	},
	{
		Slug:        "inflight-altius",
		Name:        "Altius Air Inflight",
		Category:    "inflight",
		Description: "In-flight WiFi portal with live flight chip, loyalty sign-in, buy-a-pass tiers, and free messaging (Gogo / airline style).",
		file:        "templates/inflight-altius.html",
	},
	{
		Slug:        "isp-beacon",
		Name:        "Beacon WiFi Hotspot",
		Category:    "isp",
		Description: "Carrier/ISP public hotspot: account sign-in with remember-me, plus a 'not a customer? buy a pass' path (Xfinity / Spectrum style).",
		file:        "templates/isp-beacon.html",
	},
	{
		Slug:        "generic-tos",
		Name:        "Generic Terms of Service",
		Category:    "generic",
		Description: "Clean neutral click-through portal: scrollable terms box and an 'I agree' checkbox gating the connect button.",
		file:        "templates/generic-tos.html",
	},
	{
		Slug:        "restaurant-harvest",
		Name:        "Harvest Table Kitchen",
		Category:    "restaurant",
		Description: "Casual-dining rewards-club Wi-Fi capturing first name, last name, email, birthday, and phone with a marketing opt-in (Olive Garden / Chili's style).",
		file:        "templates/restaurant-harvest.html",
	},
	{
		Slug:        "restaurant-quickbite",
		Name:        "QuickBite Burgers",
		Category:    "restaurant",
		Description: "Fast-food one-tap Wi-Fi splash with an optional receipt survey code for a free item (McDonald's / Wendy's style).",
		file:        "templates/restaurant-quickbite.html",
	},
	{
		Slug:        "retail-marketplace",
		Name:        "Marketplace Center",
		Category:    "retail",
		Description: "Shopping-mall guest Wi-Fi with a store-directory vibe: email and postal code gate with a marketing opt-in (Westfield / Simon Malls style).",
		file:        "templates/retail-marketplace.html",
	},
	{
		Slug:        "retail-bigbox",
		Name:        "MegaMart",
		Category:    "retail",
		Description: "Big-box superstore Wi-Fi: sign in with a rewards number or continue with email, scan-and-save app vibe (Target / Walmart style).",
		file:        "templates/retail-bigbox.html",
	},
	{
		Slug:        "education-university",
		Name:        "Northgate University",
		Category:    "education",
		Description: "Campus Wi-Fi with two paths: sponsored guest self-registration, or student/staff sign-in with username and password (eduroam / university IT style).",
		file:        "templates/education-university.html",
	},
	{
		Slug:        "education-k12",
		Name:        "Mapleton School District",
		Category:    "education",
		Description: "K-12 school-district visitor Wi-Fi with a content-filtering acceptable-use notice; captures visitor name and role (CIPA-style filtered network).",
		file:        "templates/education-k12.html",
	},
	{
		Slug:        "healthcare-cedar",
		Name:        "Cedar Valley Health",
		Category:    "healthcare",
		Description: "Hospital guest and visitor Wi-Fi: visitor-type selection and a HIPAA-style privacy acknowledgement before connecting (hospital guest network style).",
		file:        "templates/healthcare-cedar.html",
	},
	{
		Slug:        "library-riverside",
		Name:        "Riverside Public Library",
		Category:    "library",
		Description: "Public-library Wi-Fi: sign in with a library card number and last name, or continue as a guest (municipal library style).",
		file:        "templates/library-riverside.html",
	},
	{
		Slug:        "transit-metro",
		Name:        "MetroLink Transit",
		Category:    "transit",
		Description: "Subway and bus transit Wi-Fi splash with a live route chip and an optional email for service alerts (transit-authority style).",
		file:        "templates/transit-metro.html",
	},
	{
		Slug:        "transit-rail",
		Name:        "Coastline Rail",
		Category:    "transit",
		Description: "Intercity train onboard Wi-Fi: choose a standard or first-class tier, then enter ticket number and coach class (Amtrak / Brightline style).",
		file:        "templates/transit-rail.html",
	},
	{
		Slug:        "event-arena",
		Name:        "Apex Arena",
		Category:    "event",
		Description: "Stadium and arena event Wi-Fi with a sponsor banner: captures seat section and row plus email for event offers (sports-venue style).",
		file:        "templates/event-arena.html",
	},
	{
		Slug:        "event-expo",
		Name:        "Convene Expo Center",
		Category:    "event",
		Description: "Convention-center Wi-Fi requiring the event access code and badge number from the attendee lanyard (conference-center style).",
		file:        "templates/event-expo.html",
	},
	{
		Slug:        "hotel-resteasy",
		Name:        "RestEasy Inn & Suites",
		Category:    "hotel",
		Description: "Budget and midscale hotel Wi-Fi: simple room-number and last-name verification with an optional rewards number (Holiday Inn Express / budget style).",
		file:        "templates/hotel-resteasy.html",
	},
	{
		Slug:        "hotel-collective",
		Name:        "The Collective Hostel",
		Category:    "hotel",
		Description: "Boutique hostel and co-living Wi-Fi: bed/room code and email with a social common-room vibe (hostel style).",
		file:        "templates/hotel-collective.html",
	},
	{
		Slug:        "cruise-azure",
		Name:        "Azure Seas Cruise Line",
		Category:    "cruise",
		Description: "Cruise-ship internet packages: choose a tiered plan, then verify by stateroom and last name billed to the onboard folio (cruise-line style).",
		file:        "templates/cruise-azure.html",
	},
	{
		Slug:        "cruise-ferry",
		Name:        "BlueWave Ferries",
		Category:    "cruise",
		Description: "Passenger-ferry onboard Wi-Fi with a sailing-route chip: one-tap connect with an optional email for updates (ferry style).",
		file:        "templates/cruise-ferry.html",
	},
	{
		Slug:        "corp-azuread",
		Name:        "Northwind Traders",
		Category:    "corporate",
		Description: "Enterprise SSO sign-in: a two-step work-email then password flow with corporate tile branding (Microsoft Azure AD / Okta style).",
		file:        "templates/corp-azuread.html",
	},
	{
		Slug:        "corp-aruba",
		Name:        "Vantage Industries",
		Category:    "corporate",
		Description: "Corporate guest self-registration that creates a sponsored guest account: name, email, company, phone, and sponsor email (Aruba ClearPass style).",
		file:        "templates/corp-aruba.html",
	},
	{
		Slug:        "telecom-optimax",
		Name:        "Optimax Mobile",
		Category:    "telecom",
		Description: "National mobile-carrier hotspot sign-in with a mobile number and account PIN, plus a buy-a-pass path (carrier Wi-Fi / Boingo style).",
		file:        "templates/telecom-optimax.html",
	},
	{
		Slug:        "isp-fibernet",
		Name:        "FiberNet Communities",
		Category:    "isp",
		Description: "Residential ISP public hotspot served from home routers: subscriber email/password sign-in or buy a timed pass (Xfinity WiFi style).",
		file:        "templates/isp-fibernet.html",
	},
	{
		Slug:        "fitness-pulse",
		Name:        "Pulse Fitness Club",
		Category:    "fitness",
		Description: "Gym member Wi-Fi sign-in with membership ID and PIN, plus a day-pass guest email path (health-club chain style).",
		file:        "templates/fitness-pulse.html",
	},
	{
		Slug:        "automotive-voltway",
		Name:        "VoltWay Charging",
		Category:    "automotive",
		Description: "EV charging-station lounge Wi-Fi: email and station id with an offers opt-in while you charge (EV-network style).",
		file:        "templates/automotive-voltway.html",
	},
	{
		Slug:        "automotive-tripstop",
		Name:        "TripStop Travel Center",
		Category:    "automotive",
		Description: "Highway travel-center and truck-stop Wi-Fi: rewards number or email with a fuel-rewards vibe (Pilot / Love's style).",
		file:        "templates/automotive-tripstop.html",
	},
	{
		Slug:        "social-cityconnect",
		Name:        "CityConnect Free WiFi",
		Category:    "social",
		Description: "Municipal public-Wi-Fi splash with email or phone verification and simulated social-login buttons (city / social-login splash style).",
		file:        "templates/social-cityconnect.html",
	},
}

// Catalog returns all built-in templates with HTML loaded from the embedded FS, sorted by category then name.
func Catalog() ([]Template, error) {
	out := make([]Template, 0, len(catalog))
	for _, t := range catalog {
		b, err := templateFS.ReadFile(t.file)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", t.slugFile(), err)
		}
		t.HTML = string(b)
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (t Template) slugFile() string {
	if t.Slug != "" {
		return t.Slug
	}
	return t.file
}
