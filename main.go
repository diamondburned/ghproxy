package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/diamondburned/ghproxy/htmlmut"
	"github.com/diamondburned/ghproxy/proxy"
	"github.com/diamondburned/listener"
)

func main() {
	username := os.Getenv("GHPROXY_USERNAME")
	if username == "" {
		log.Fatalln("Missing $GHPROXY_USERNAME.")
	}

	address := os.Getenv("GHPROXY_ADDRESS")
	if address == "" {
		log.Fatalln("Missing $GHPROXY_ADDRESS.")
	}

	githubRoot := url.URL{Scheme: "https", Host: "github.com"}
	githubURL := githubRoot
	githubURL.Path = "/" + username

	// Replace all hyperlinks with the appropriate proxied ones.
	linkReplacer := func(link string) string {
		// Trim the GitHub domain for the code below to work.
		link = strings.TrimPrefix(link, githubRoot.String())
		// Don't handle links that are not relative. This includes non-GitHub
		// links.
		if len(link) == 0 || link[0] != '/' {
			return link
		}
		// Any link that does NOT go to our profile MUST go to the official
		// GitHub page.
		if !strings.HasPrefix(link, githubURL.Path) {
			return "https://github.com" + link
		}
		// Keep using our proxy otherwise.
		link = strings.TrimPrefix(link, githubURL.Path)
		if link == "" {
			// Empty path means root.
			return "/"
		}
		return link
	}

	rp := proxy.NewReverseProxy(githubURL, htmlmut.ChainMutators(
		htmlmut.TagRemover("signup-prompt"),
		htmlmut.TagRemover("header"),
		htmlmut.AnchorReplace(linkReplacer),
		htmlmut.TagAttrReplace("a", "data-hovercard-url", linkReplacer),
		// Source: https://github.com/DuBistKomisch/disable-pjax-github
		htmlmut.ScriptInjector(`
			document.body.addEventListener("pjax:click", ev => ev.preventDefault())
		`),
		// Minor appearance tweaks.
		htmlmut.CSSInjector(`
			main > div.mt-4:first-child {
				margin-top:  0px !important;
				padding-top: 6px !important;
			}
		`),
	))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		cancel()
	}()

	httpLogger := log.New(os.Stderr, "[http]", log.LstdFlags)

	server := &http.Server{
		Addr:         address,
		Handler:      rp,
		ReadTimeout:  10 * time.Second,
		IdleTimeout:  8 * time.Second,
		WriteTimeout: 5 * time.Second,
		ErrorLog:     httpLogger,
	}

	if err := listener.HTTPListenAndServeCtx(ctx, server); err != nil {
		log.Fatalln("Failed to serve HTTP:", err)
	}
}
