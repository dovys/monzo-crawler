package crawler

import (
	"context"
	"net/url"
	"sync"
)

type Crawler interface {
	Enqueue(*url.URL)
	Run(context.Context) (<-chan *Page, <-chan error)
}

type Page struct {
	url.URL
	Links  []*url.URL
	Assets []*url.URL
}

type CrawlerOption func(*crawler)

func NewCrawler(p Parser, f Fetcher, u UniqueSet, options ...CrawlerOption) Crawler {
	c := &crawler{
		queue:       make(chan *url.URL, 100000000),
		concurrency: 5,
		parser:      p,
		fetcher:     f,
		uniqueSet:   u,
		wg:          sync.WaitGroup{},
	}

	for _, f := range options {
		f(c)
	}

	return c
}

func Concurrency(limit int) CrawlerOption {
	return func(c *crawler) {
		c.concurrency = limit
	}
}

type crawler struct {
	queue       chan *url.URL
	concurrency int
	wg          sync.WaitGroup
	parser      Parser
	fetcher     Fetcher
	uniqueSet   UniqueSet
}

func (c *crawler) Enqueue(u *url.URL) {
	// Making sure to not crawl the same page more than once
	if !c.uniqueSet.AddIfNotExists(u) {
		return
	}

	c.wg.Add(1)
	c.queue <- u
}

func (c *crawler) Run(ctx context.Context) (<-chan *Page, <-chan error) {
	// 1000?
	errors := make(chan error, c.concurrency)
	results := make(chan *Page, c.concurrency)
	// Finished is used to stop goroutines after the queue is emptied
	finished := make(chan bool)
	// Running is used to make sure all goroutines are finished before the results and errors
	// channels are closed so we don't end up writing to a closed channel.
	running := sync.WaitGroup{}

	go func() {
		c.wg.Wait()
		close(finished)
	}()

	for i := 0; i < c.concurrency; i++ {
		go func() {
			running.Add(1)
			defer running.Done()

			for {
				select {
				// Queue is empty
				case <-finished:
					return
				// Cancelled
				case <-ctx.Done():
					return

				// Work to be done
				case u := <-c.queue:
					page, err := c.crawl(u)
					if err != nil {
						errors <- err
						break
					}

					results <- page

					for _, link := range page.Links {
						c.Enqueue(link)
					}

					c.wg.Done()
				}
			}
		}()
	}

	go func() {
		running.Wait()
		close(results)
		close(errors)
	}()

	return results, errors
}

func (c *crawler) crawl(u *url.URL) (*Page, error) {
	b, err := c.fetcher.Fetch(u.String())
	if err != nil {
		return nil, err
	}

	links, assets := c.parser.Parse(u, b)
	linksOnSameHost := make([]*url.URL, 0)
	for _, link := range links {
		if link.Host == u.Host {
			linksOnSameHost = append(linksOnSameHost, link)
		}
	}

	return &Page{
		URL:    *u,
		Links:  linksOnSameHost,
		Assets: assets,
	}, nil
}
