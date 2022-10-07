package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_urlize(t *testing.T) {
	assert.Equal(t, "bc-ef", urlize("äbc ef"))
	assert.Equal(t, "this-is-a-test", urlize("This Is A Test"))
}

func Fuzz_urlize(f *testing.F) {
	f.Add("Test")
	f.Fuzz(func(t *testing.T, str string) {
		out := urlize(str)
		if out == "" {
			t.Error("Empty output")
		}
	})
}

func Benchmark_urlize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		urlize("äbc ef")
	}
}

func Test_sortedStrings(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, sortedStrings([]string{"a", "c", "b"}))
}

func Test_generateRandomString(t *testing.T) {
	assert.Len(t, randomString(30), 30)
	assert.Len(t, randomString(50), 50)
}

func Test_isAbsoluteURL(t *testing.T) {
	assert.True(t, isAbsoluteURL("http://example.com"))
	assert.True(t, isAbsoluteURL("https://example.com"))
	assert.False(t, isAbsoluteURL("/test"))
}

func Test_wordCount(t *testing.T) {
	assert.Equal(t, 3, wordCount("abc def abc"))
}

func Benchmark_wordCount(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wordCount("abc def abc")
	}
}

func Test_charCount(t *testing.T) {
	assert.Equal(t, 4, charCount("  t  e\n  s  t €.☺️"))
}

func Benchmark_charCount(b *testing.B) {
	for i := 0; i < b.N; i++ {
		charCount("  t  e\n  s  t €.☺️")
	}
}

func Test_allLinksFromHTMLString(t *testing.T) {
	baseUrl := "https://example.net/post/abc"
	html := `<a href="relative1">Test</a><a href="relative1">Test</a><a href="/relative2">Test</a><a href="https://example.com">Test</a>`
	expected := []string{"https://example.net/post/relative1", "https://example.net/relative2", "https://example.com"}

	result, err := allLinksFromHTMLString(html, baseUrl)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func Test_urlHasExt(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		ext, res := urlHasExt("https://example.com/test.jpg", "png", "jpg", "webp")
		assert.True(t, res)
		assert.Equal(t, "jpg", ext)
	})
	t.Run("Strange case", func(t *testing.T) {
		ext, res := urlHasExt("https://example.com/test.jpG", "PnG", "JPg", "WEBP")
		assert.True(t, res)
		assert.Equal(t, "jpg", ext)
	})
}

func Test_htmlText(t *testing.T) {
	// Text without HTML
	assert.Equal(t, "This is a test", htmlText("This is a test"))
	// Text without HTML and Emojis
	assert.Equal(t, "This is a test 😁", htmlText("This is a test 😁"))
	// Text without HTML and quoutes
	assert.Equal(t, "This is a 'test'", htmlText("This is a 'test'"))
	// Text with formatting (like bold or italic)
	assert.Equal(t, "This is a test", htmlText("<b>This is a test</b>"))
	assert.Equal(t, "This is a test", htmlText("<i>This is a test</i>"))
	// Unordered list
	assert.Equal(t, "Test\n\nTest", htmlText(`<ul><li>Test</li><li>Test</li></ul>`))
	// Ordered list
	assert.Equal(t, "1. Test\n\n2. Test", htmlText(`<ol><li>Test</li><li>Test</li></ol>`))
	// Nested unordered list
	assert.Equal(t, "Test\n\nTest\n\nTest", htmlText(`<ul><li>Test</li><li><ul><li>Test</li><li>Test</li></ul></li></ul>`))
	// Headers and paragraphs
	assert.Equal(t, "Test\n\nTest", htmlText(`<h1>Test</h1><p>Test</p>`))
	assert.Equal(t, "Test\n\nTest\n\nTest", htmlText(`<h1>Test</h1><p>Test</p><h2>Test</h2>`))
	// Blockquote
	assert.Equal(t, "Test\n\nBlockqoute content", htmlText(`<p>Test</p><blockquote><p>Blockqoute content</p></blockquote>`))
	// Nested blockquotes
	assert.Equal(t, "Blockqoute content\n\nBlockqoute content", htmlText(`<blockquote><p>Blockqoute content</p><blockquote><p>Blockqoute content</p></blockquote></blockquote>`))
	// Code (should be ignored)
	assert.Equal(t, "Test", htmlText(`<p>Test</p><pre><code>Code content</code></pre>`))
	// Inline code (should not be ignored)
	assert.Equal(t, "Test Code content", htmlText(`<p>Test <code>Code content</code></p>`))
}

func Test_containsStrings(t *testing.T) {
	assert.True(t, containsStrings("Test", "xx", "es", "st"))
	assert.False(t, containsStrings("Test", "xx", "aa"))
}

func Test_defaultIfEmpty(t *testing.T) {
	assert.Equal(t, "def", defaultIfEmpty("", "def"))
	assert.Equal(t, "first", defaultIfEmpty("first", "def"))
}

func Test_matchTimeDiffLocale(t *testing.T) {
	assert.Equal(t, "en", string(matchTimeDiffLocale("en-US")))
	assert.Equal(t, "en", string(matchTimeDiffLocale("en")))
	assert.Equal(t, "de", string(matchTimeDiffLocale("de")))
	assert.Equal(t, "de", string(matchTimeDiffLocale("de-DE")))
	assert.Equal(t, "de", string(matchTimeDiffLocale("de-AT")))
	assert.Equal(t, "pt", string(matchTimeDiffLocale("pt-BR")))
	assert.Equal(t, "pt", string(matchTimeDiffLocale("pt")))
}

func Test_unescapedPath(t *testing.T) {
	assert.Equal(t, "/de/posts/fahrradanhänger", unescapedPath("/de/posts/fahrradanh%C3%A4nger"))
	assert.Equal(t, "/de/posts/fahrradanhänger", unescapedPath("/de/posts/fahrradanhänger"))
}

func Test_lowerUnescaptedPath(t *testing.T) {
	assert.Equal(t, "/de/posts/fahrradanhänger", lowerUnescapedPath("/de/posts/fahrradanh%C3%84nger"))
	assert.Equal(t, "/de/posts/fahrradanhänger", lowerUnescapedPath("/de/posts/fahrradanhÄnger"))
}

func Fuzz_lowerUnescaptedPath(f *testing.F) {
	f.Add("/de/posts/fahrradanh%C3%84nger")
	f.Fuzz(func(t *testing.T, str string) {
		out := lowerUnescapedPath(str)
		if out == "" {
			t.Error("Empty output")
		}
	})
}

func Test_groupStrings(t *testing.T) {
	strings := []string{"Aaaaaa", "Dddedddee", "Bbbbb", "anjkdhfkjshf", "hjgsfkjdhkfhskjdfh", "🚴🏼‍♀️ jhfjshkfjh"}
	groups := groupStrings(strings)

	assert.Len(t, groups, 5)

	assert.Equal(t, "A", groups[0].Identifier)
	assert.Equal(t, "B", groups[1].Identifier)
	assert.Equal(t, "D", groups[2].Identifier)
	assert.Equal(t, "H", groups[3].Identifier)
	assert.Equal(t, "🚴", groups[4].Identifier)
}
