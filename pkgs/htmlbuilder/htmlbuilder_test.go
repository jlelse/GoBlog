package htmlbuilder

import (
	"bytes"
	"io"
	"testing"
)

var _ io.Writer = &HTMLBuilder{}
var _ io.StringWriter = &HTMLBuilder{}

func TestHTMLBuilder(t *testing.T) {
	var buf bytes.Buffer
	hb := NewHTMLBuilder(&buf)
	hb.WriteElementOpen("div", "class", "test \"quote\"", "data-n", 42, "data-b", true)
	hb.WriteEscaped("a & b < c > d 'e' \"f\"\x00")
	hb.WriteElementClose("div")

	want := `<div class="test &#34;quote&#34;" data-n=42 data-b=true>a &amp; b &lt; c &gt; d &#39;e&#39; &#34;f&#34;` + "\uFFFD" + `</div>`
	if got := buf.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func BenchmarkHTMLBuilder(b *testing.B) {
	var buf bytes.Buffer
	hb := NewHTMLBuilder(&buf)
	b.ReportAllocs()
	for b.Loop() {
		buf.Reset()
		hb.WriteElementOpen("article", "class", "h-entry border-bottom", "data-id", 12)
		hb.WriteElementOpen("h2", "class", "p-name")
		hb.WriteElementOpen("a", "href", "/posts/hello-world")
		hb.WriteEscaped("Hello, World!")
		hb.WriteElementsClose("a", "h2", "article")
	}
}
