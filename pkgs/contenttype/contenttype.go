package contenttype

// This package contains constants for a few content types used in GoBlog

const (
	CharsetUtf8Suffix = "; charset=utf-8"

	AS            = "application/activity+json"
	ATOM          = "application/atom+xml"
	CSS           = "text/css"
	HTML          = "text/html"
	JPEG          = "image/jpeg"
	JS            = "application/javascript"
	JSON          = "application/json"
	JSONFeed      = "application/feed+json"
	LDJSON        = "application/ld+json"
	MultipartForm = "multipart/form-data"
	PNG           = "image/png"
	RSS           = "application/rss+xml"
	Text          = "text/plain"
	WWWForm       = "application/x-www-form-urlencoded"
	XML           = "text/xml"

	ASUTF8   = AS + CharsetUtf8Suffix
	CSSUTF8  = CSS + CharsetUtf8Suffix
	HTMLUTF8 = HTML + CharsetUtf8Suffix
	JSONUTF8 = JSON + CharsetUtf8Suffix
	JSUTF8   = JS + CharsetUtf8Suffix
	TextUTF8 = Text + CharsetUtf8Suffix
	XMLUTF8  = XML + CharsetUtf8Suffix
)
