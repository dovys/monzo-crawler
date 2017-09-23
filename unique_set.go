package crawler

import (
	"net/url"
	"sync"

	"github.com/OneOfOne/xxhash"
)

// UniqueSet is a thread safe add-only hashSet which helps to make sure
// we don't process the same url more than once. It assumes urls
// with the same host, path & query, but different #fragment are identical.
type UniqueSet interface {
	// Returns false if the url already exists in the set
	AddIfNotExists(*url.URL) bool
}

func NewUniqueSet() UniqueSet {
	return &syncUniqueSet{
		log: make(map[uint64]struct{}, 0),
		mu:  sync.Mutex{},
	}
}

type syncUniqueSet struct {
	log map[uint64]struct{}
	mu  sync.Mutex
}

func (s *syncUniqueSet) AddIfNotExists(u *url.URL) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	chsum := xxhash.ChecksumString64(u.Host + u.Path + u.RawQuery)

	if _, exists := s.log[chsum]; exists {
		return false
	}

	s.log[chsum] = struct{}{}

	return true
}
