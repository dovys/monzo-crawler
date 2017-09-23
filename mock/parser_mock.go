package mock

import (
	"net/url"

	"github.com/stretchr/testify/mock"
)

type ParserMock struct {
	mock.Mock
}

func (p *ParserMock) Parse(root *url.URL, body []byte) (links, assets []*url.URL) {
	args := p.Called(root, body)

	return args.Get(0).([]*url.URL), args.Get(1).([]*url.URL)
}
