package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"

	crawler "github.com/dovys/monzo-crawler"
)

type Config struct {
	HTTPTimeout        time.Duration `envconfig:"http_timeout" default:"10s"`
	Concurrency        int           `envconfig:"concurrency" default:"5"`
	ResultBufferLength int           `envconfig:"result_buffer" default:"5"`
	MaxQueueLength     int           `envconfig:"queue_length" default:"5"`
}

type pageResult struct {
	URL    string   `json:"url"`
	Links  []string `json:"links"`
	Assets []string `json:"assets"`
}

func main() {
	var cfg Config
	envconfig.MustProcess("", &cfg)

	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <url>\n", os.Args[0])
		os.Exit(1)
	}

	uri, err := url.Parse(os.Args[1])
	if err != nil {
		fmt.Println(errors.Wrap(err, "Invalid url"))
		os.Exit(1)
	}

	if uri.Scheme != "https" && uri.Scheme != "http" {
		fmt.Println("Supported schemes: http, https.")
		os.Exit(1)
	}

	h := &http.Client{
		Timeout: cfg.HTTPTimeout,
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

		crawler.Concurrency(cfg.Concurrency),
		crawler.ResultBufferLength(cfg.ResultBufferLength),
		crawler.MaxQueueLength(cfg.MaxQueueLength),
	)

	c.Enqueue(uri)

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

	time.Sleep(2 * time.Second) // remove
}
