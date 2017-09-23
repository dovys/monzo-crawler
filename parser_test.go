package crawler

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var body = `
	<html>
		<head>
			<link rel="stylesheet" href="/main.css">
			<script src="bundle.js"></script>			
		</head>
		<body>
			<a href="/articles/1" class="articles"><span class="divier">Articles</span></a>
			
			<a href="https://mydomain.com/account">Acount</a>
			<a href='http://mydomain.com/images'></a>
			<a href="//www.youtube.com/?"></a>
			<a href=""></a>
			<a href="/home">
				<img src="/home.jpg" />
			</a>

			<img src='http://imgur.com/abcdef.jpg' class="img" />
			<script async src="//www.google-analytics.com/analytics.js"></script>			
		</body>
	</html>
`

func TestParseLinks(t *testing.T) {
	p := NewParser()

	root, _ := url.Parse("https://mydomain.com/page/1")
	links, _ := p.Parse(root, []byte(body))

	expected := []string{
		"https://mydomain.com/articles/1",
		"https://mydomain.com/account",
		"http://mydomain.com/images",
		"https://www.youtube.com/?",
		"https://mydomain.com/home",
	}

	for i := 0; i < len(expected); i++ {
		assert.Equal(t, expected[i], links[i].String())
	}
}

func TestParseAssets(t *testing.T) {
	p := NewParser()

	root, _ := url.Parse("https://mydomain.com/page/1")
	_, assets := p.Parse(root, []byte(body))

	require.Len(t, assets, 5)
	expected := []string{
		"https://mydomain.com/main.css",
		"https://mydomain.com/page/bundle.js",
		"https://mydomain.com/home.jpg",
		"http://imgur.com/abcdef.jpg",
		"https://www.google-analytics.com/analytics.js",
	}

	for i := 0; i < len(expected); i++ {
		assert.Equal(t, expected[i], assets[i].String())
	}
}

func TestParseRelativeURLs(t *testing.T) {
	p := NewParser()

	body := `<html><body><a href="issues/351">351</a></body></html>`

	uri, _ := url.Parse("https://mydomain.com/issues")
	links, _ := p.Parse(uri, []byte(body))

	require.Len(t, links, 1)
	assert.Equal(t, "https://mydomain.com/issues/351", links[0].String())
}
