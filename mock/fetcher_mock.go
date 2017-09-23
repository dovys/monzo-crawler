package mock

import "github.com/stretchr/testify/mock"

type FetcherMock struct {
	mock.Mock
}

func (f *FetcherMock) Fetch(url string) ([]byte, error) {
	args := f.Called(url)

	return args.Get(0).([]byte), args.Error(1)
}
