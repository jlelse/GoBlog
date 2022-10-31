package webrings

import (
	"fmt"

	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

func GetPlugin() plugintypes.UI {
	return &plugin{}
}

type plugin struct {
	config map[string]any
}

func (*plugin) SetApp(_ plugintypes.App) {
	// Ignore
}

func (p *plugin) SetConfig(config map[string]any) {
	p.config = config
}

func (p *plugin) Render(hb *htmlbuilder.HtmlBuilder, t plugintypes.RenderType, data plugintypes.RenderData, render plugintypes.RenderNextFunc) {
	render(hb)
	if t == plugintypes.BlogFooterRenderType {
		bd, ok := data.(plugintypes.BlogRenderData)
		if !ok {
			fmt.Println("webrings plugin: data is not BlogRenderData!")
			return
		}
		blogData := bd.GetBlog()
		if blogData == nil {
			fmt.Println("webrings plugin: blog is nil!")
			return
		}
		blog := blogData.GetBlog()
		if blog == "" {
			fmt.Println("webrings plugin: blog is empty!")
			return
		}
		if blogWebringsAny, ok := p.config[blog]; ok {
			if blogWebrings, ok := blogWebringsAny.([]any); ok {
				for _, webringAny := range blogWebrings {
					if webring, ok := webringAny.(map[string]any); ok {
						title, titleOk := unwrapToString(webring["title"])
						prev, prevOk := unwrapToString(webring["prev"])
						next, nextOk := unwrapToString(webring["next"])
						if titleOk && (prevOk || nextOk) {
							hb.WriteElementOpen("p")
							if prevOk {
								hb.WriteElementOpen("a", "href", prev)
								hb.WriteEscaped("←")
								hb.WriteElementClose("a")
							}
							hb.WriteEscaped(" " + title + " ")
							if nextOk {
								hb.WriteElementOpen("a", "href", next)
								hb.WriteEscaped("→")
								hb.WriteElementClose("a")
							}
							hb.WriteElementClose("p")
						}
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
