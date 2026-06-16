// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, paid training, paid CTF,
// or any for-profit use requires a license from VTEM Labs. See the LICENSE file.

package portal

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	scrapeUA          = "Mozilla/5.0 (X11; Linux aarch64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
	scrapeHTTPTimeout = 20 * time.Second
	maxPageBytes      = 2 << 20 // 2 MB of HTML
	maxAssetBytes     = 2 << 20 // 2 MB per inlined asset
	maxAssetCount     = 40
	// Raw asset budget; base64 inflates ~33%, keeping page+inline under the portals.html field cap.
	maxAssetTotal = 4 << 20
)

// Scrape fetches rawURL, inlines its stylesheets and images, drops external scripts, and Normalizes the result.
// Outbound fetches are SSRF-guarded against internal/cloud-metadata addresses. Returns self-contained HTML.
func Scrape(rawURL string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") {
		return "", fmt.Errorf("invalid URL (use http:// or https://)")
	}

	client := ssrfSafeClient()
	page, _, err := fetchLimited(client, base.String(), maxPageBytes)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}

	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return "", fmt.Errorf("parse failed: %w", err)
	}

	// Honour <base href> for resolving relative references, then drop it so absolute rewrites aren't resolved twice.
	refBase := base
	var stylesheets, images, scripts, baseEls []*html.Node
	var others []*html.Node // nodes with a relative href/src to absolutize
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Base:
				if href := getAttr(n, "href"); href != "" {
					if u, e := url.Parse(href); e == nil {
						refBase = base.ResolveReference(u)
					}
				}
				baseEls = append(baseEls, n)
			case atom.Link:
				if strings.Contains(strings.ToLower(getAttr(n, "rel")), "stylesheet") && getAttr(n, "href") != "" {
					stylesheets = append(stylesheets, n)
				} else {
					others = append(others, n)
				}
			case atom.Img:
				images = append(images, n)
			case atom.Script:
				if getAttr(n, "src") != "" {
					scripts = append(scripts, n)
				}
			case atom.A:
				others = append(others, n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	budget := &assetBudget{remaining: maxAssetTotal}

	// Inline stylesheets: <link> -> <style>.
	for _, n := range stylesheets {
		abs, ok := resolveURL(refBase, getAttr(n, "href"))
		if !ok {
			continue
		}
		css, _, e := fetchLimited(client, abs, maxAssetBytes)
		if e != nil || !budget.take(int64(len(css))) {
			continue
		}
		n.DataAtom = atom.Style
		n.Data = "style"
		n.Attr = nil
		for c := n.FirstChild; c != nil; { // clear any children (link is void)
			next := c.NextSibling
			n.RemoveChild(c)
			c = next
		}
		n.AppendChild(&html.Node{Type: html.TextNode, Data: string(css)})
	}

	// Inline images as data URIs.
	for _, n := range images {
		src := getAttr(n, "src")
		if src == "" || strings.HasPrefix(src, "data:") {
			continue
		}
		abs, ok := resolveURL(refBase, src)
		if !ok {
			continue
		}
		data, ct, e := fetchLimited(client, abs, maxAssetBytes)
		if e != nil || !budget.take(int64(len(data))) {
			// Could not inline; at least make the reference absolute.
			setAttr(n, "src", abs)
			continue
		}
		setAttr(n, "src", dataURI(ct, data))
		// srcset would re-introduce network refs.
		removeAttr(n, "srcset")
	}

	// Drop external scripts entirely (avoid calling home / breaking offline).
	for _, n := range scripts {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}

	// Absolutize remaining relative links so nothing points at a bare path.
	for _, n := range others {
		for _, key := range []string{"href", "src"} {
			if v := getAttr(n, key); v != "" {
				if abs, ok := resolveURL(refBase, v); ok {
					setAttr(n, key, abs)
				}
			}
		}
	}

	// Remove <base> so the absolute URLs above are final.
	for _, n := range baseEls {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
		}
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return "", fmt.Errorf("render failed: %w", err)
	}
	return Normalize(buf.String()), nil
}

type assetBudget struct {
	remaining int64
	count     int
}

func (b *assetBudget) take(n int64) bool {
	if b.count >= maxAssetCount || n > b.remaining {
		return false
	}
	b.remaining -= n
	b.count++
	return true
}

// ssrfSafeClient returns an HTTP client whose dialer Control hook rejects connections to non-public IPs,
// blocking loopback/private/link-local/CGNAT even via DNS rebinding or redirects.
func ssrfSafeClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if blockedIP(net.ParseIP(host)) {
				return fmt.Errorf("blocked non-public address: %s", host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: scrapeHTTPTimeout,
		Transport: &http.Transport{
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			DisableKeepAlives:   true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to a non-http(s) URL is blocked")
			}
			return nil
		},
	}
}

// blockedIP reports whether an IP must not be connected to from the scraper.
func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// Carrier-grade NAT 100.64.0.0/10.
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast()
}

func fetchLimited(client *http.Client, u string, max int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", scrapeUA)
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max))
	if err != nil {
		return nil, "", err
	}
	return b, resp.Header.Get("Content-Type"), nil
}

func resolveURL(base *url.URL, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	low := strings.ToLower(ref)
	if ref == "" || strings.HasPrefix(low, "data:") || strings.HasPrefix(ref, "#") ||
		strings.HasPrefix(low, "javascript:") || strings.HasPrefix(low, "mailto:") ||
		strings.HasPrefix(low, "tel:") {
		return "", false
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", false
	}
	abs := base.ResolveReference(u)
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return "", false
	}
	return abs.String(), true
}

func dataURI(contentType string, b []byte) string {
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(b)
}

func removeAttr(n *html.Node, key string) {
	out := n.Attr[:0]
	for _, a := range n.Attr {
		if !strings.EqualFold(a.Key, key) {
			out = append(out, a)
		}
	}
	n.Attr = out
}
