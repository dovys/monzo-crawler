package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"

	stdmock "github.com/stretchr/testify/mock"

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

	s.f.On("Fetch", root.String()).Once().Return([]byte("body"), nil)
	s.f.On("Fetch", link.String()).Once().Return([]byte("aboutBody"), nil)
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
	s := setup(1, 100, 100)

	root, _ := url.Parse("https://google.com")
	external, _ := url.Parse("https://twitter.com/handle")

	s.f.On("Fetch", root.String()).Once().Return([]byte("body"), nil)
	s.p.On("Parse", root, []byte("body")).Return([]*url.URL{root, external}, []*url.URL{})

	s.c.Enqueue(root)

	run(s.c, root, context.Background())

	s.AssertExpectations(t)
}

func TestLinksAndAssets(t *testing.T) {
	s := setup(1, 100, 100)

	root, _ := url.Parse("https://google.com")
	link, _ := url.Parse("https://google.com/about")
	assetJs, _ := url.Parse("https://google.com/bundle.js")
	assetImg, _ := url.Parse("https://google.com/img.png")
	assetImg2, _ := url.Parse("https://google.com/img2.png")

	s.f.On("Fetch", root.String()).Return([]byte("body"), nil)
	s.f.On("Fetch", link.String()).Return([]byte("bodyAbout"), nil)
	s.p.On("Parse", root, []byte("body")).Return([]*url.URL{link}, []*url.URL{assetImg, assetJs})
	s.p.On("Parse", link, []byte("bodyAbout")).Return([]*url.URL{root}, []*url.URL{assetImg2, assetJs})

	s.c.Enqueue(root)

	pages, _ := run(s.c, root, context.Background())

	assert.Len(t, pages, 2)

	assert.Equal(t, []*url.URL{link}, pages[0].Links)
	assert.Equal(t, []*url.URL{root}, pages[1].Links)
	assert.Equal(t, []*url.URL{assetImg, assetJs}, pages[0].Assets)
	assert.Equal(t, []*url.URL{assetImg2, assetJs}, pages[1].Assets)

	s.AssertExpectations(t)
}

func TestHTTPErrorsDontStopExecution(t *testing.T) {
	s := setup(1, 100, 100)
	root, _ := url.Parse("https://google.com")
	errorDepth2, _ := url.Parse("https://google.com/error")
	okDepth2, _ := url.Parse("https://google.com/about")
	okDepth3, _ := url.Parse("https://google.com/about/more")
	okDepth4, _ := url.Parse("https://google.com/about/more/even_more")

	err := &HTTPError{StatusCode: http.StatusBadRequest, Message: "msg"}

	s.f.On("Fetch", root.String()).Return([]byte("body"), nil)
	s.f.On("Fetch", okDepth2.String()).Return([]byte("depth2Body"), nil)
	s.f.On("Fetch", errorDepth2.String()).Return([]byte("errorBody"), err)
	s.f.On("Fetch", okDepth3.String()).Return([]byte("depth3Body"), nil)
	s.f.On("Fetch", okDepth4.String()).Return([]byte("depth4Body"), nil)

	s.p.On("Parse", root, []byte("body")).Return([]*url.URL{errorDepth2, okDepth2}, []*url.URL{})
	s.p.On("Parse", okDepth2, []byte("depth2Body")).Return([]*url.URL{okDepth3}, []*url.URL{})
	s.p.On("Parse", okDepth3, []byte("depth3Body")).Return([]*url.URL{okDepth4}, []*url.URL{})
	s.p.On("Parse", okDepth4, []byte("depth4Body")).Return([]*url.URL{}, []*url.URL{})

	s.c.Enqueue(root)

	pages, errors := run(s.c, root, context.Background())

	assert.Len(t, pages, 4)
	assert.Len(t, errors, 1)

	assert.Equal(t, err, errors[0])
}

func TestCancellationLetsCurrentCrawlsFinish(t *testing.T) {
	s := setup(1, 100, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	root, _ := url.Parse("https://google.com")
	about, _ := url.Parse("https://google.com/about")
	tos, _ := url.Parse("https://google.com/tos")

	s.f.On("Fetch", root.String()).Return([]byte("body"), nil)
	s.f.On("Fetch", about.String()).Run(func(a stdmock.Arguments) {
		cancel()
	}).Return([]byte("body"), nil)

	s.p.On("Parse", root, []byte("body")).Return([]*url.URL{about}, []*url.URL{})
	s.p.On("Parse", about, []byte("body")).Return([]*url.URL{tos}, []*url.URL{})

	s.c.Enqueue(root)

	p, _ := run(s.c, root, ctx)

	assert.Len(t, p, 2)
	assert.Equal(t, root.String(), p[0].String())
	assert.Equal(t, about.String(), p[1].String())

	s.AssertExpectations(t)
}

func TestCrawlerDiscardsNewItemsInQueueWhenFull(t *testing.T) {
	s := setup(1, 1, 10)

	root, _ := url.Parse("https://google.com")
	about, _ := url.Parse("https://google.com/about")
	tos, _ := url.Parse("https://google.com/tos")
	sitemap, _ := url.Parse("https://google.com/sitemap")

	s.f.On("Fetch", root.String()).Return([]byte("body"), nil)
	s.f.On("Fetch", about.String()).Return([]byte("body"), nil)

	s.p.On("Parse", root, []byte("body")).Return([]*url.URL{about, tos, sitemap}, []*url.URL{})
	s.p.On("Parse", about, []byte("body")).Return([]*url.URL{root}, []*url.URL{})

	s.c.Enqueue(root)

	p, e := run(s.c, root, context.Background())

	assert.Len(t, p, 2)
	assert.Equal(t, root.String(), p[0].String())
	assert.Equal(t, about.String(), p[1].String())

	assert.Len(t, e, 2)
	assert.Equal(t, ErrQueueLimitReached, e[0])
	assert.Equal(t, ErrQueueLimitReached, e[1])

	s.AssertExpectations(t)
}

func run(c Crawler, root *url.URL, ctx context.Context) ([]*Page, []error) {
	pagechn, errchn := c.Run(ctx)

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
