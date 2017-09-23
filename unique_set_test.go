package crawler

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	s := NewUniqueSet()

	assert.True(t, s.AddIfNotExists(&url.URL{Host: "domain.com"}))
	assert.False(t, s.AddIfNotExists(&url.URL{Host: "domain.com"}))
	assert.False(t, s.AddIfNotExists(&url.URL{Host: "domain.com"}))
}

func TestURLFragmentsAreIgnored(t *testing.T) {
	s := NewUniqueSet()

	root, _ := url.Parse("https://www.facebook.com/home")
	assert.True(t, s.AddIfNotExists(root))

	anchored, _ := url.Parse("https://www.facebook.com/home#jump-to-headline")
	assert.False(t, s.AddIfNotExists(anchored))
}

func BenchmarkAdd(b *testing.B) {
	s := NewUniqueSet()

	for i := 0; i < b.N; i++ {
		s.AddIfNotExists(&url.URL{Host: "mydomain.com/" + string(i)})
	}
}
