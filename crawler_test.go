package crawler

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"testing"

	"github.com/dovys/monzo-crawler/mock"
	"github.com/stretchr/testify/assert"
)

type testSuite struct {
	c Crawler
	p *mock.ParserMock
	f *mock.FetcherMock
}

func (s *testSuite) AssertExpectations(t *testing.T) {
	s.f.AssertExpectations(t)
	s.p.AssertExpectations(t)
}

func setup(concurrencyLimit, maxQueueLength, resultBufferLength int) *testSuite {
	p := &mock.ParserMock{}
	f := &mock.FetcherMock{}

	opts := []CrawlerOption{
		Concurrency(concurrencyLimit),
		MaxQueueLength(maxQueueLength),
		ResultBufferLength(resultBufferLength),
	}

	c := NewCrawler(p, f, NewUniqueSet(), opts...)

	return &testSuite{c: c, p: p, f: f}
}

func TestSamePageIsCrawledOnce(t *testing.T) {
	s := setup(1, 100, 100)
	root, _ := url.Parse("https://google.com")
	link, _ := url.Parse("https://google.com/about")

	s.f.On("Fetch", "https://google.com").Once().Return([]byte("body"), nil)
	s.f.On("Fetch", "https://google.com/about").Once().Return([]byte("aboutBody"), nil)
	s.p.On("Parse", root, []byte("body")).Return([]*url.URL{root, link, root, root}, []*url.URL{})
	s.p.On("Parse", link, []byte("aboutBody")).Return([]*url.URL{root, link}, []*url.URL{})

	s.c.Enqueue(root)

	run(s.c, root, context.Background())

	s.AssertExpectations(t)
}

func TestConcurrentCrawls(t *testing.T) {
	s := setup(10, 100, 100)
	pages := make([]*url.URL, 26)

	for i := 0; i < 26; i++ {
		pages[i], _ = url.Parse(fmt.Sprintf("https://google.com/page-%s", string(byte(i+'a'))))
		s.f.On("Fetch", pages[i].String()).Once().Return([]byte("body"), nil)
	}

	for i := 0; i < 26; i++ {
		s.p.On("Parse", pages[i], []byte("body")).Once().Return(pages, []*url.URL{})
	}

	s.c.Enqueue(pages[0])

	p, _ := run(s.c, pages[0], context.Background())

	assert.Len(t, p, 26)

	s.AssertExpectations(t)
}

func TestExternalLinksAreNotFollowed(t *testing.T) {

}

func TestLinksAndAssets(t *testing.T) {

}

func TestCancellation(t *testing.T) {

}

func TestCrawlerDiscardsNewItemsInQueueWhenFull(t *testing.T) {

}

func run(c Crawler, root *url.URL, ctx context.Context) ([]*Page, []error) {
	pagechn, errchn := c.Run(context.Background())

	pages := make([]*Page, 0)
	errors := make([]error, 0)

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		for p := range pagechn {
			pages = append(pages, p)
		}
	}()

	go func() {
		defer wg.Done()
		for e := range errchn {
			errors = append(errors, e)
		}
	}()

	wg.Wait()

	return pages, errors
}
