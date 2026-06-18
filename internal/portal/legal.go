// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package portal

import (
	"fmt"
	"strings"
)

// legalPages maps a URL path to the generic legal document served there so portal Terms/AUP/Privacy links resolve.
var legalPages = map[string]legalDoc{
	"/legal/terms":   legalTerms,
	"/legal/aup":     legalAUP,
	"/legal/privacy": legalPrivacy,
}

type legalDoc struct {
	Title    string
	Sections []legalSection
}

type legalSection struct {
	Heading string
	Body    string
}

// LegalPageHTML renders the legal document registered at the given path and reports whether it exists.
func LegalPageHTML(path string) (string, bool) {
	doc, ok := legalPages[path]
	if !ok {
		return "", false
	}
	return legalPageHTML(doc), true
}

// legalPageHTML renders one legal document as a standalone, self-contained HTML page.
func legalPageHTML(doc legalDoc) string {
	var b strings.Builder
	for _, s := range doc.Sections {
		b.WriteString(`<h2>` + s.Heading + `</h2>`)
		b.WriteString(`<p>` + s.Body + `</p>`)
	}
	return fmt.Sprintf(legalShell, doc.Title, doc.Title, b.String())
}

const legalShell = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
  :root { color-scheme: light; }
  * { box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #f3f4f6; color: #1f2937; margin: 0; line-height: 1.65; }
  .wrap { max-width: 720px; margin: 0 auto; padding: 32px 20px 64px; }
  .bar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 24px; }
  .back { color: #2563eb; text-decoration: none; font-size: 14px; font-weight: 600; }
  .back:hover { text-decoration: underline; }
  .doc { background: #fff; border: 1px solid #e5e7eb; border-radius: 12px;
    padding: 32px 36px; box-shadow: 0 1px 3px rgba(0,0,0,0.06); }
  h1 { font-size: 24px; margin: 0 0 4px; color: #111827; }
  .eff { color: #6b7280; font-size: 13px; margin: 0 0 24px; }
  h2 { font-size: 16px; color: #111827; margin: 24px 0 6px; }
  p { margin: 0 0 12px; color: #374151; font-size: 14px; }
  .foot { color: #9ca3af; font-size: 12px; margin-top: 28px; text-align: center; }
</style>
</head>
<body>
  <div class="wrap">
    <div class="bar">
      <a class="back" href="/">&larr; Back to Wi-Fi sign-in</a>
    </div>
    <div class="doc">
      <h1>%s</h1>
      <p class="eff">This is a generic guest network policy provided for reference.</p>
      %s
      <p class="foot">If you do not agree with this policy, please disconnect from the network.</p>
    </div>
  </div>
</body>
</html>`

var legalTerms = legalDoc{
	Title: "Terms of Service",
	Sections: []legalSection{
		{"1. Acceptance of Terms", "By accessing this wireless network you agree to these Terms of Service. If you do not agree, do not connect to or use the network."},
		{"2. Service Provided As-Is", "Network access is provided as a courtesy, on an as-is and as-available basis, without warranties of any kind. The network operator does not guarantee availability, speed, security, or fitness for any particular purpose."},
		{"3. Permitted Use", "The network is intended for lawful personal and business use. You agree to use it in compliance with all applicable laws and with the Acceptable Use Policy."},
		{"4. No Liability", "To the maximum extent permitted by law, the network operator is not liable for any direct, indirect, incidental, or consequential damages arising from your use of, or inability to use, the network, including data loss or unauthorized access to data you transmit."},
		{"5. Security", "This is an open wireless network. Traffic may be observed by others. You are responsible for protecting your own device and data, and should avoid transmitting sensitive information over open Wi-Fi."},
		{"6. Termination", "The operator may suspend or terminate access at any time, for any reason, without notice."},
		{"7. Changes", "These terms may be updated from time to time. Continued use of the network constitutes acceptance of the current terms."},
	},
}

var legalAUP = legalDoc{
	Title: "Acceptable Use Policy",
	Sections: []legalSection{
		{"1. Lawful Use Only", "You may use this network only for lawful purposes. Any activity that violates local, national, or international law is prohibited."},
		{"2. Prohibited Activities", "You may not use the network to transmit malware; attempt to gain unauthorized access to any system; interfere with or disrupt the network or other users; send unsolicited bulk messages (spam); or infringe intellectual property rights."},
		{"3. Prohibited Content", "You may not use the network to access, store, or distribute content that is unlawful, harassing, defamatory, or otherwise objectionable, including child sexual abuse material, which will be reported to authorities."},
		{"4. Bandwidth and Fair Use", "Network capacity is shared. Excessive consumption, including sustained high-volume downloading or streaming that degrades service for others, may be rate-limited or blocked."},
		{"5. Monitoring", "Network usage may be monitored and logged to maintain security and enforce this policy. Suspected abuse may result in immediate termination of access."},
		{"6. Responsibility", "You are responsible for all activity conducted through your device on this network. Keep your credentials and device secure."},
	},
}

var legalPrivacy = legalDoc{
	Title: "Privacy Policy",
	Sections: []legalSection{
		{"1. Information We Collect", "To provide and secure network access, we may collect technical information such as your device MAC address, IP address, browser user agent, and connection timestamps, along with any information you submit on the sign-in page."},
		{"2. How We Use Information", "Collected information is used to authenticate access, operate and secure the network, enforce the Acceptable Use Policy, and comply with legal obligations."},
		{"3. Information Sharing", "We do not sell your personal information. Information may be disclosed to service providers who help operate the network, or to authorities where required by law."},
		{"4. Data Retention", "Connection and usage records are retained only as long as necessary for the purposes described above, or as required by applicable law."},
		{"5. Open Network Notice", "This network may be unencrypted. Information transmitted over an open wireless network can be intercepted by third parties. Avoid transmitting sensitive personal or financial information."},
		{"6. Contact", "Questions about this policy can be directed to the network operator at the venue where this network is provided."},
	},
}
