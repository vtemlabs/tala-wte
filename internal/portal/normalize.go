// Tala WTE - Wireless Training Environment
// Copyright (c) 2026 VTEM Labs. All rights reserved.
// Free for personal and non-profit use. Commercial, for-profit, and government
// use require a license from VTEM Labs. The Software may not be copied or
// redistributed. See the LICENSE file.

package portal

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Normalize rewrites arbitrary captive-portal HTML so its forms POST to /portal/accept with a redirect field
// and recognized credential field names, appending a Connect form if none exists. Idempotent; pure markup rewriting.
func Normalize(htmlStr string) string {
	if strings.TrimSpace(htmlStr) == "" {
		return htmlStr
	}
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}

	var forms []*html.Node
	var body *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Form:
				forms = append(forms, n)
			case atom.Body:
				body = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if len(forms) == 0 {
		if body != nil {
			body.AppendChild(buildConnectForm())
		}
	} else {
		for _, f := range forms {
			normalizeForm(f)
		}
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return htmlStr
	}
	return buf.String()
}

const defaultRedirect = "http://connectivitycheck.gstatic.com/generate_204"

// normalizeForm wires a single <form> to the accept endpoint.
func normalizeForm(f *html.Node) {
	setAttr(f, "method", "post")
	setAttr(f, "action", "/portal/accept")

	var inputs []*html.Node
	var pwd *html.Node
	hasRedirect := false
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Input {
			inputs = append(inputs, n)
			if strings.EqualFold(getAttr(n, "name"), "redirect") {
				hasRedirect = true
			}
			if pwd == nil && strings.EqualFold(getAttr(n, "type"), "password") {
				pwd = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(f)

	// Tag the password field and the first plausible username field before it, unless already recognized.
	if pwd != nil {
		if !recognizedField(getAttr(pwd, "name"), credPassKeys) {
			setAttr(pwd, "name", "password")
		}
		for _, in := range inputs {
			if in == pwd {
				break
			}
			switch strings.ToLower(getAttr(in, "type")) {
			case "hidden", "submit", "button", "checkbox", "radio", "image", "file":
				continue
			}
			if !recognizedField(getAttr(in, "name"), credUserKeys) {
				setAttr(in, "name", "username")
			}
			break
		}
	}

	if !hasRedirect {
		f.InsertBefore(hiddenInput("redirect", defaultRedirect), f.FirstChild)
	}
}

func recognizedField(name string, keys []string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, k := range keys {
		if name == k {
			return true
		}
	}
	return false
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, val string) {
	for i := range n.Attr {
		if strings.EqualFold(n.Attr[i].Key, key) {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func hiddenInput(name, val string) *html.Node {
	return &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Input,
		Data:     "input",
		Attr: []html.Attribute{
			{Key: "type", Val: "hidden"},
			{Key: "name", Val: name},
			{Key: "value", Val: val},
		},
	}
}

func buildConnectForm() *html.Node {
	form := &html.Node{
		Type: html.ElementNode, DataAtom: atom.Form, Data: "form",
		Attr: []html.Attribute{{Key: "method", Val: "post"}, {Key: "action", Val: "/portal/accept"}},
	}
	form.AppendChild(hiddenInput("redirect", defaultRedirect))
	btn := &html.Node{
		Type: html.ElementNode, DataAtom: atom.Button, Data: "button",
		Attr: []html.Attribute{{Key: "type", Val: "submit"}},
	}
	btn.AppendChild(&html.Node{Type: html.TextNode, Data: "Connect to Internet"})
	form.AppendChild(btn)
	return form
}
