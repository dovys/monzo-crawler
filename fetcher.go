package crawler

import (
	"io/ioutil"
	"net/http"
)

type Fetcher interface {
	Fetch(url string) ([]byte, error)
}

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e HTTPError) Error() string {
	return e.Message
}

type fetcher struct {
	httpClient *http.Client
}

func NewFetcher(c *http.Client) Fetcher {
	return &fetcher{httpClient: c}
}

func (f *fetcher) Fetch(url string) ([]byte, error) {
	rsp, err := f.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	b, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, err
	}

	// 3XX's are handled by the http client
	if rsp.StatusCode != 200 {
		return nil, &HTTPError{StatusCode: rsp.StatusCode, Message: string(b)}
	}

	return b, nil
}
