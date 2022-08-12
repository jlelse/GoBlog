package htmlbuilder

import "io"

var _ io.Writer = &HtmlBuilder{}
var _ io.StringWriter = &HtmlBuilder{}
