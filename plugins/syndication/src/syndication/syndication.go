package syndication

import (
	"github.com/PuerkitoBio/goquery"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app           plugintypes.App
	parameterName string
}

func GetPlugin() (plugintypes.SetConfig, plugintypes.SetApp, plugintypes.UI2) {
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

func (p *plugin) RenderWithDocument(rc plugintypes.RenderContext, doc *goquery.Document) {
	post, err := p.app.GetPost(rc.GetPath())
	if err != nil || post == nil {
		return
	}
	syndicationLinks, ok := post.GetParameters()[p.parameterName]
	if !ok || len(syndicationLinks) == 0 {
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
}
