// Package proxy provides a more featureful abstraction on top of
// httputil.ReverseProxy.
package proxy

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/diamondburned/ghproxy/htmlmut"
)

// writeBigError writes a big red HTML error. This will appear very ugly.
func writeBigError(w io.Writer, err error) {
	const tmpl = `<h1 class="proxy-error" style="color:red;font-family:monospace">%s</h1>`
	fmt.Fprintf(w, tmpl, html.EscapeString(err.Error()))
}

type ReverseProxy struct {
	*httputil.ReverseProxy
	targetURL         *url.URL
	htmlMutator       HTMLMutator
	cookieInterceptor CookieInterceptor
}

func NewReverseProxy(target url.URL, htmlMutator htmlmut.MutateFunc) *ReverseProxy {
	domainHeader := fmt.Sprintf("Domain=%s; ", target.Hostname())
	targetURL := &target

	if htmlMutator == nil {
		htmlMutator = htmlmut.ChainMutators()
	}

	return &ReverseProxy{
		ReverseProxy: httputil.NewSingleHostReverseProxy(targetURL),
		targetURL:    targetURL,
		htmlMutator:  NewHTMLMutator(htmlMutator),
		// There isn't a better way, unfortunately. Set-Cookie headers are
		// parsed differently.
		cookieInterceptor: NewCookieInterceptor(func(setCookie string) string {
			return strings.ReplaceAll(setCookie, domainHeader, "")
		}),
	}
}

// ServeHTTP serves the reverse proxy. If the request has a path that starts
// with the previously given targetURL, the server will 301 redirect that to a
// request with the path trimmed.
func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If the path contains the path prefix that we want to strip, then 301
	// redirect it to the right path.
	if strings.HasPrefix(r.URL.Path, rp.targetURL.Path) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, rp.targetURL.Path)
		http.Redirect(w, r, r.URL.String(), http.StatusMovedPermanently)
		return
	}

	cWriter := rp.cookieInterceptor.NewWriter(w)
	htmlMut := rp.htmlMutator.NewWriter(cWriter)
	r.Host = rp.targetURL.Host

	rp.ReverseProxy.ServeHTTP(htmlMut, r)

	if err := htmlMut.ApplyHTML(); err != nil {
		writeBigError(w, err)
	}
}
