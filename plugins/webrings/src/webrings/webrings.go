package webrings

import (
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

func GetPlugin() (plugintypes.SetConfig, plugintypes.UIFooter) {
	p := &plugin{}
	return p, p
}

type plugin struct {
	config map[string]any
}

func (p *plugin) SetConfig(config map[string]any) {
	p.config = config
}

func (p *plugin) RenderFooter(rc plugintypes.RenderContext, doc *goquery.Document) {
	blog := rc.GetBlog()
	if blog == "" {
		fmt.Println("webrings plugin: blog is empty!")
		return
	}
	if blogWebringsAny, ok := p.config[blog]; ok {
		if blogWebrings, ok := blogWebringsAny.([]any); ok {
			buf := bufferpool.Get()
			defer bufferpool.Put(buf)
			hb := htmlbuilder.NewHtmlBuilder(buf)
			for _, webringAny := range blogWebrings {
				if webring, ok := webringAny.(map[string]any); ok {
					title, titleOk := unwrapToString(webring["title"])
					link, linkOk := unwrapToString(webring["link"])
					prev, prevOk := unwrapToString(webring["prev"])
					next, nextOk := unwrapToString(webring["next"])
					if titleOk && (linkOk || prevOk || nextOk) {
						buf.Reset()
						hb.WriteElementOpen("p")
						if prevOk {
							hb.WriteElementOpen("a", "href", prev)
							hb.WriteEscaped("←")
							hb.WriteElementClose("a")
							hb.WriteEscaped(" ")
						}
						if linkOk {
							hb.WriteElementOpen("a", "href", link)
						}
						hb.WriteEscaped(title)
						if linkOk {
							hb.WriteElementClose("a")
						}
						if nextOk {
							hb.WriteEscaped(" ")
							hb.WriteElementOpen("a", "href", next)
							hb.WriteEscaped("→")
							hb.WriteElementClose("a")
						}
						hb.WriteElementClose("p")
						doc.Find("footer").AppendHtml(buf.String())
					}
				}
			}
		}
	}
}

func unwrapToString(o any) (string, bool) {
	if o == nil {
		return "", false
	}
	s, ok := o.(string)
	return s, ok && s != ""
}
