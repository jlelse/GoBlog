// Package contenttype defines common content type constants.
package contenttype

const (
	// CharsetUtf8Suffix is the UTF-8 charset suffix for content types.
	CharsetUtf8Suffix = "; charset=utf-8"

	AS            = "application/activity+json"  // AS is the ActivityPub JSON content type.
	ATOM          = "application/atom+xml"       // ATOM is the Atom feed content type.
	CSS           = "text/css"                   // CSS is the CSS content type.
	HTML          = "text/html"                  // HTML is the HTML content type.
	JPEG          = "image/jpeg"                 // JPEG is the JPEG image content type.
	JS            = "application/javascript"     // JS is the JavaScript content type.
	JSON          = "application/json"           // JSON is the JSON content type.
	JSONFeed      = "application/feed+json"      // JSONFeed is the JSON Feed content type.
	LDJSON        = "application/ld+json"        // LDJSON is the JSON-LD content type.
	MultipartForm = "multipart/form-data"        // MultipartForm is the multipart form content type.
	PNG           = "image/png"                  // PNG is the PNG image content type.
	RSS           = "application/rss+xml"        // RSS is the RSS feed content type.
	Text          = "text/plain"                 // Text is the plain text content type.
	WWWForm       = "application/x-www-form-urlencoded" // WWWForm is the URL-encoded form content type.
	XML           = "text/xml"                   // XML is the XML content type.

	ASUTF8   = AS + CharsetUtf8Suffix
	CSSUTF8  = CSS + CharsetUtf8Suffix
	HTMLUTF8 = HTML + CharsetUtf8Suffix
	JSONUTF8 = JSON + CharsetUtf8Suffix
	JSUTF8   = JS + CharsetUtf8Suffix
	TextUTF8 = Text + CharsetUtf8Suffix
	XMLUTF8  = XML + CharsetUtf8Suffix
)
