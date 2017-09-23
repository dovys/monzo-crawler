package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	crawler "github.com/dovys/monzo-crawler"
)

type pageResult struct {
	URL    string   `json:"url"`
	Links  []string `json:"links"`
	Assets []string `json:"assets"`
}

func main() {
	// concurrency limit used as a throttling mechanism instead of doing req/s
	h := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Preventing redirects to a different host
			for _, v := range via {
				if req.URL.Host != v.URL.Host {
					return http.ErrUseLastResponse
				}
			}

			return nil
		},
	}

	c := crawler.NewCrawler(
		crawler.NewParser(),
		crawler.NewFetcher(h),
		crawler.NewUniqueSet(),
	)

	root, err := url.Parse("http://steamcommunity.com/market/")
	// root, err := url.Parse("https://getacorn.com")
	if err != nil {
		panic(err)
	}

	c.Enqueue(root)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pages, errors := c.Run(ctx)
	go func() {
		logger := log.New(os.Stderr, "", log.LstdFlags)
		for err := range errors {
			if err == crawler.ErrTooManyRequests {
				logger.Println("Stopping due to too many requests")
				cancel()
			}

			logger.Println(err)
		}
	}()

	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")

	for page := range pages {
		p := pageResult{
			URL:    page.String(),
			Links:  make([]string, len(page.Links)),
			Assets: make([]string, len(page.Assets)),
		}

		for i := 0; i < len(page.Links); i++ {
			p.Links[i] = page.Links[i].String()
		}

		for i := 0; i < len(page.Assets); i++ {
			p.Assets[i] = page.Assets[i].String()
		}

		e.Encode(p)
	}

	time.Sleep(2 * time.Second)
}
