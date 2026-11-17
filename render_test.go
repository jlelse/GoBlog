package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.goblog.app/app/pkgs/plugintypes"
)

type testUIPlugin struct {
	suffix string
	panic  bool
}

func (p testUIPlugin) Render(_ plugintypes.RenderContext, rendered io.Reader, modified io.Writer) {
	if p.panic {
		panic("ui plugin panic")
	}
	_, _ = io.Copy(modified, rendered)
	_, _ = io.WriteString(modified, p.suffix)
}

func TestChainUIPlugins(t *testing.T) {
	app := &goBlog{}
	plugins := []any{
		testUIPlugin{suffix: "<one>"},
		testUIPlugin{suffix: "<two>"},
	}
	var out bytes.Buffer
	app.chainUIPlugins(plugins, &pluginRenderContext{}, strings.NewReader("original"), &out)
	assert.Equal(t, "original<one><two>", out.String())
}

func TestChainUIPluginsContinuesAfterPanic(t *testing.T) {
	app := &goBlog{}
	plugins := []any{
		testUIPlugin{suffix: "<one>"},
		testUIPlugin{panic: true},
		testUIPlugin{suffix: "<three>"},
	}
	var out bytes.Buffer
	app.chainUIPlugins(plugins, &pluginRenderContext{}, strings.NewReader("original"), &out)
	assert.Equal(t, "original<one><three>", out.String())
}
