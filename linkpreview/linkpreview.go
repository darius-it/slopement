// Package linkpreview fetches a page and reads what it says about itself (its
// OpenGraph title, description and picture) from this device, for the
// setting "Fetch previews from this device". The alternative, the
// homeserver's preview_url, hides the user's address from the site; this
// shows it, which is why it is a choice.
//
// A link in a message is the sender's, so the fetch is careful: only http and
// https, never an address on the user's own network or machine (a link could
// otherwise make this device probe a router or a local service), a small size
// and a short time.
package linkpreview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"

	"slopement/model"
)

const (
	maxPage    = 1 << 20 // how much of a page is read
	maxImage   = 4 << 20 // the largest picture fetched
	maxRedirs  = 5
	fetchLimit = 12 * time.Second
)

// ErrBlocked is returned for an address the fetch will not go to.
var ErrBlocked = errors.New("linkpreview: that address is on a private network")

// Client fetches previews. The zero value is ready to use.
type Client struct {
	// AllowPrivate lets it reach private addresses: for tests with a server
	// on this machine, never for the app.
	AllowPrivate bool
	// HTTP overrides the HTTP client (tests).
	HTTP *http.Client
}

// blocked reports an address that is not on the public internet.
func blocked(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	for _, p := range []string{"100.64.0.0/10", "192.0.0.0/24", "198.18.0.0/15", "240.0.0.0/4", "fec0::/10"} {
		if netip.MustParsePrefix(p).Contains(ip) {
			return true
		}
	}
	return false
}

// control is the dialer's check, run on the address a connection is about to
// be made to — after the name was resolved, so a name that points inward is
// caught too, and on every redirect.
func control(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || blocked(ip) {
		return ErrBlocked
	}
	return nil
}

func (c *Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	d := &net.Dialer{Timeout: 8 * time.Second}
	if !c.AllowPrivate {
		d.Control = control
	}
	return &http.Client{
		Timeout:   fetchLimit,
		Transport: &http.Transport{DialContext: d.DialContext, Proxy: nil, ResponseHeaderTimeout: 8 * time.Second, DisableKeepAlives: true},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirs {
				return errors.New("too many redirects")
			}
			return checkScheme(req.URL)
		},
	}
}

func checkScheme(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("linkpreview: %q is not a web address", u.Scheme)
	}
	if u.Hostname() == "" {
		return errors.New("linkpreview: no host")
	}
	return nil
}

func (c *Client) get(ctx context.Context, raw, accept string, limit int64) (body []byte, final *url.URL, ctype string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, nil, "", err
	}
	if err := checkScheme(u); err != nil {
		return nil, nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, "", err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Slopement link preview)")
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, "", fmt.Errorf("linkpreview: %s", resp.Status)
	}
	if resp.ContentLength > limit && resp.ContentLength > 0 && limit == maxImage {
		return nil, nil, "", errors.New("linkpreview: the picture is too big")
	}
	body, err = io.ReadAll(io.LimitReader(resp.Body, limit))
	return body, resp.Request.URL, resp.Header.Get("Content-Type"), err
}

// Fetch reads a page's preview: its title, description, site name and a
// picture's address. A page with nothing to say returns an empty Preview and
// no error; a page that is not HTML is an error.
func (c *Client) Fetch(ctx context.Context, link string) (model.Preview, error) {
	body, final, ctype, err := c.get(ctx, link, "text/html,application/xhtml+xml;q=0.9", maxPage)
	if err != nil {
		return model.Preview{}, err
	}
	if mt := strings.ToLower(strings.SplitN(ctype, ";", 2)[0]); mt != "" && mt != "text/html" && mt != "application/xhtml+xml" {
		return model.Preview{}, fmt.Errorf("linkpreview: not a web page (%s)", mt)
	}
	return parse(body, ctype, final), nil
}

// Image fetches a preview's picture (at most 4 MB).
func (c *Client) Image(ctx context.Context, link string) ([]byte, error) {
	body, _, ctype, err := c.get(ctx, link, "image/*", maxImage)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(strings.ToLower(ctype), "image/") {
		return nil, errors.New("linkpreview: not a picture")
	}
	return body, nil
}

// parse reads the tags of a page's head. Pure, for the tests.
func parse(body []byte, ctype string, base *url.URL) model.Preview {
	r, err := charset.NewReader(strings.NewReader(string(body)), ctype)
	if err != nil {
		r = strings.NewReader(string(body))
	}
	doc, err := html.Parse(r)
	if err != nil {
		return model.Preview{}
	}
	meta := map[string]string{}
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Meta:
				key := attr(n, "property")
				if key == "" {
					key = attr(n, "name")
				}
				if key = strings.ToLower(key); key != "" {
					if _, seen := meta[key]; !seen { // the first wins
						meta[key] = strings.TrimSpace(attr(n, "content"))
					}
				}
			case atom.Title:
				if title == "" && n.FirstChild != nil {
					title = strings.TrimSpace(n.FirstChild.Data)
				}
			case atom.Body:
				return // everything wanted is in the head
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)
	first := func(keys ...string) string {
		for _, k := range keys {
			if v := meta[k]; v != "" {
				return v
			}
		}
		return ""
	}
	p := model.Preview{
		URL:         base.String(),
		Site:        first("og:site_name"),
		Title:       first("og:title", "twitter:title"),
		Description: first("og:description", "twitter:description", "description"),
	}
	if p.Title == "" {
		p.Title = title
	}
	if p.Site == "" {
		p.Site = strings.TrimPrefix(base.Hostname(), "www.")
	}
	if img := first("og:image:secure_url", "og:image", "twitter:image", "twitter:image:src"); img != "" {
		if ref, err := url.Parse(img); err == nil {
			if abs := base.ResolveReference(ref); abs.Scheme == "https" || abs.Scheme == "http" {
				p.Image = abs.String()
				p.W, _ = strconv.Atoi(meta["og:image:width"])
				p.H, _ = strconv.Atoi(meta["og:image:height"])
			}
		}
	}
	p.Title, p.Description = clip(p.Title, 200), clip(p.Description, 400)
	return p
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// clip shortens s to at most n runes.
func clip(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n-1]) + "…"
}
