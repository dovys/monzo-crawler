package crawler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
)

var (
	ErrTooManyRequests   = errors.New("Too many requests")
	ErrQueueLimitReached = errors.New("Queue limit reached")
)

type Crawler interface {
	Enqueue(*url.URL) error
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
		queue:              make(chan *url.URL, 1000000),
		concurrency:        5,
		parser:             p,
		fetcher:            f,
		uniqueSet:          u,
		wg:                 sync.WaitGroup{},
		resultBufferLength: 100,
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

func MaxQueueLength(length int) CrawlerOption {
	return func(c *crawler) {
		c.queue = make(chan *url.URL, length)
	}
}

func ResultBufferLength(length int) CrawlerOption {
	return func(c *crawler) {
		c.resultBufferLength = length
	}
}

type crawler struct {
	concurrency        int
	resultBufferLength int

	queue     chan *url.URL
	wg        sync.WaitGroup
	parser    Parser
	fetcher   Fetcher
	uniqueSet UniqueSet
}

func (c *crawler) Enqueue(u *url.URL) error {
	// Making sure to not crawl the same page more than once
	if !c.uniqueSet.AddIfNotExists(u) {
		return nil
	}

	select {
	case c.queue <- u:
		c.wg.Add(1)
	default:
		return ErrQueueLimitReached
	}

	return nil
}

func (c *crawler) Run(ctx context.Context) (<-chan *Page, <-chan error) {
	errors := make(chan error, c.resultBufferLength)
	results := make(chan *Page, c.resultBufferLength)
	// Finished is used to stop goroutines after the queue is emptied
	finished := make(chan bool)
	// Running is used to make sure all goroutines are finished before the results and errors
	// channels are closed so we don't end up writing to a closed channel.
	running := sync.WaitGroup{}

	go func() {
		c.wg.Wait()
		close(finished)
	}()

	running.Add(c.concurrency)
	for i := 0; i < c.concurrency; i++ {
		go func() {
			defer running.Done()

			for {
				// Prioritising cancellation
				select {
				// Cancelled
				case <-ctx.Done():
					return
				default:
					select {
					// Queue is empty
					case <-finished:
						return

					// Work to be done
					case u := <-c.queue:
						page, err := c.crawl(u)
						if err != nil {
							errors <- err
							c.wg.Done()
							break
						}

						results <- page

						for i := 0; i < len(page.Links); i++ {
							if err := c.Enqueue(page.Links[i]); err != nil {
								errors <- err
							}
						}

						c.wg.Done()
					}
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
		if e, ok := err.(*HTTPError); ok && e.StatusCode == http.StatusTooManyRequests {
			return nil, ErrTooManyRequests
		}

		return nil, err
	}

	links, assets := c.parser.Parse(u, b)
	linksOnSameHost := make([]*url.URL, 0)
	for i := 0; i < len(links); i++ {
		if links[i].Host == u.Host {
			linksOnSameHost = append(linksOnSameHost, links[i])
		}
	}

	return &Page{
		URL:    *u,
		Links:  linksOnSameHost,
		Assets: assets,
	}, nil
}
