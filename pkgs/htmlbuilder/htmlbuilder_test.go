package htmlbuilder

import "io"

var _ io.Writer = &HTMLBuilder{}
var _ io.StringWriter = &HTMLBuilder{}
