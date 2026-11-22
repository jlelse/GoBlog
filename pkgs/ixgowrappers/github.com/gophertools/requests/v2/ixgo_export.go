// export by github.com/goplus/ixgo/cmd/qexp

package requests

import (
	q "github.com/gophertools/requests/v2"

	"go/constant"
	"reflect"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterPackage(&ixgo.Package{
		Name: "requests",
		Path: "github.com/gophertools/requests/v2",
		Deps: map[string]string{
			"bufio":            "bufio",
			"bytes":            "bytes",
			"cmp":              "cmp",
			"compress/gzip":    "gzip",
			"context":          "context",
			"encoding/base64":  "base64",
			"encoding/json":    "json",
			"encoding/json/v2": "json",
			"errors":           "errors",
			"fmt":              "fmt",
			"github.com/gophertools/requests/v2/internal/minitrue": "minitrue",
			"golang.org/x/net/publicsuffix":                        "publicsuffix",
			"io":                                                   "io",
			"maps":                                                 "maps",
			"mime":                                                 "mime",
			"mime/multipart":                                       "multipart",
			"net/http":                                             "http",
			"net/http/cookiejar":                                   "cookiejar",
			"net/url":                                              "url",
			"os":                                                   "os",
			"path/filepath":                                        "filepath",
			"regexp":                                               "regexp",
			"slices":                                               "slices",
			"strconv":                                              "strconv",
			"strings":                                              "strings",
			"time":                                                 "time",
		},
		Interfaces: map[string]reflect.Type{},
		NamedTypes: map[string]reflect.Type{
			"Builder":       reflect.TypeOf((*q.Builder)(nil)).Elem(),
			"ErrorKind":     reflect.TypeOf((*q.ErrorKind)(nil)).Elem(),
			"ResponseError": reflect.TypeOf((*q.ResponseError)(nil)).Elem(),
			"RoundTripFunc": reflect.TypeOf((*q.RoundTripFunc)(nil)).Elem(),
		},
		AliasTypes: map[string]reflect.Type{
			"BodyGetter":          reflect.TypeOf((*q.BodyGetter)(nil)).Elem(),
			"CheckRedirectPolicy": reflect.TypeOf((*q.CheckRedirectPolicy)(nil)).Elem(),
			"Config":              reflect.TypeOf((*q.Config)(nil)).Elem(),
			"Deserializer":        reflect.TypeOf((*q.Deserializer)(nil)).Elem(),
			"ResponseHandler":     reflect.TypeOf((*q.ResponseHandler)(nil)).Elem(),
			"Serializer":          reflect.TypeOf((*q.Serializer)(nil)).Elem(),
			"Transport":           reflect.TypeOf((*q.Transport)(nil)).Elem(),
		},
		Vars: map[string]reflect.Value{
			"DefaultValidator":   reflect.ValueOf(&q.DefaultValidator),
			"ErrInvalidHandled":  reflect.ValueOf(&q.ErrInvalidHandled),
			"JSONDeserializer":   reflect.ValueOf(&q.JSONDeserializer),
			"JSONSerializer":     reflect.ValueOf(&q.JSONSerializer),
			"JSONV1Deserializer": reflect.ValueOf(&q.JSONV1Deserializer),
			"JSONV1Serializer":   reflect.ValueOf(&q.JSONV1Serializer),
			"NoFollow":           reflect.ValueOf(&q.NoFollow),
		},
		Funcs: map[string]reflect.Value{
			"BodyMultipart":      reflect.ValueOf(q.BodyMultipart),
			"BodySerializer":     reflect.ValueOf(q.BodySerializer),
			"ChainHandlers":      reflect.ValueOf(q.ChainHandlers),
			"CheckContentType":   reflect.ValueOf(q.CheckContentType),
			"CheckPeek":          reflect.ValueOf(q.CheckPeek),
			"CheckStatus":        reflect.ValueOf(q.CheckStatus),
			"DoerTransport":      reflect.ValueOf(q.DoerTransport),
			"ErrorJSON":          reflect.ValueOf(q.ErrorJSON),
			"ErrorTransport":     reflect.ValueOf(q.ErrorTransport),
			"GzipConfig":         reflect.ValueOf(q.GzipConfig),
			"HasStatusErr":       reflect.ValueOf(q.HasStatusErr),
			"LogTransport":       reflect.ValueOf(q.LogTransport),
			"MaxFollow":          reflect.ValueOf(q.MaxFollow),
			"New":                reflect.ValueOf(q.New),
			"NewCookieJar":       reflect.ValueOf(q.NewCookieJar),
			"PermitURLTransport": reflect.ValueOf(q.PermitURLTransport),
			"ToBufioReader":      reflect.ValueOf(q.ToBufioReader),
			"ToBufioScanner":     reflect.ValueOf(q.ToBufioScanner),
			"ToBytesBuffer":      reflect.ValueOf(q.ToBytesBuffer),
			"ToDeserializer":     reflect.ValueOf(q.ToDeserializer),
			"URL":                reflect.ValueOf(q.URL),
			"UserAgentTransport": reflect.ValueOf(q.UserAgentTransport),
			"ValidatorHandler":   reflect.ValueOf(q.ValidatorHandler),
		},
		TypedConsts: map[string]ixgo.TypedConst{
			"ErrHandler":   {Typ: reflect.TypeOf(q.ErrHandler), Value: constant.MakeInt64(int64(q.ErrHandler))},
			"ErrRequest":   {Typ: reflect.TypeOf(q.ErrRequest), Value: constant.MakeInt64(int64(q.ErrRequest))},
			"ErrTransport": {Typ: reflect.TypeOf(q.ErrTransport), Value: constant.MakeInt64(int64(q.ErrTransport))},
			"ErrURL":       {Typ: reflect.TypeOf(q.ErrURL), Value: constant.MakeInt64(int64(q.ErrURL))},
			"ErrValidator": {Typ: reflect.TypeOf(q.ErrValidator), Value: constant.MakeInt64(int64(q.ErrValidator))},
		},
		UntypedConsts: map[string]ixgo.UntypedConst{},
	})
}
