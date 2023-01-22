package syndication

import (
	"fmt"
	"io"

	"github.com/PuerkitoBio/goquery"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app           plugintypes.App
	parameterName string
}

func GetPlugin() (plugintypes.SetConfig, plugintypes.SetApp, plugintypes.UI) {
	p := &plugin{}
	return p, p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) SetConfig(config map[string]any) {
	p.parameterName = "syndication" // default
	if configParameterAny, ok := config["parameter"]; ok {
		if configParameter, ok := configParameterAny.(string); ok {
			p.parameterName = configParameter // override default from config
		}
	}
}

func (p *plugin) Render(rc plugintypes.RenderContext, rendered io.Reader, modified io.Writer) {
	def := func() {
		_, _ = io.Copy(modified, rendered)
	}
	post, err := p.app.GetPost(rc.GetPath())
	if err != nil || post == nil {
		def()
		return
	}
	syndicationLinks, ok := post.GetParameters()[p.parameterName]
	if !ok || len(syndicationLinks) == 0 {
		def()
		return
	}
	doc, err := goquery.NewDocumentFromReader(rendered)
	if err != nil {
		fmt.Println("syndication plugin: " + err.Error())
		def()
		return
	}
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	hb := htmlbuilder.NewHtmlBuilder(buf)
	for _, link := range syndicationLinks {
		hb.WriteElementOpen("data", "value", link, "class", "u-syndication hide")
		hb.WriteElementClose("data")
	}
	doc.Find("main.h-entry article").AppendHtml(buf.String())
	_ = goquery.Render(modified, doc.Selection)
}
