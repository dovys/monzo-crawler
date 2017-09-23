package crawler

import (
	"bytes"
	"net/url"

	"golang.org/x/net/html"
)

type Parser interface {
	Parse(root *url.URL, body []byte) (links, assets []*url.URL)
}

func NewParser() Parser {
	return &htmlParser{}
}

type htmlParser struct{}

func (p *htmlParser) Parse(root *url.URL, body []byte) (links, assets []*url.URL) {
	t := html.NewTokenizer(bytes.NewReader(body))

	for {
		tp := t.Next()

		switch {
		case tp == html.ErrorToken:
			return
		case tp == html.StartTagToken:
			token := t.Token()

			switch token.Data {
			case "a":
				href := extractAttr("href", &token)
				if href == "" {
					break
				}

				if link, err := url.Parse(href); err == nil {
					links = append(links, root.ResolveReference(link))
				}

			case "script":
				src := extractAttr("src", &token)
				if src == "" {
					break
				}

				if link, err := url.Parse(src); err == nil {
					assets = append(assets, root.ResolveReference(link))
				}
			case "link":
				href := extractAttr("href", &token)
				if href == "" {
					break
				}

				if link, err := url.Parse(href); err == nil {
					assets = append(assets, root.ResolveReference(link))
				}
			}

		case tp == html.SelfClosingTagToken:
			token := t.Token()

			if token.Data != "img" {
				break
			}

			src := extractAttr("src", &token)
			if src == "" {
				break
			}
			if link, err := url.Parse(src); err == nil {
				assets = append(assets, root.ResolveReference(link))
			}
		}
	}
}

func extractAttr(key string, t *html.Token) string {
	for _, a := range t.Attr {
		if a.Key == key {
			return a.Val
		}
	}

	return ""
}
