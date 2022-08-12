package syndication

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
	switch t {
	case plugintypes.PostMainElementRenderType:
		render(hb)
		pd, ok := data.(plugintypes.PostRenderData)
		if !ok {
			fmt.Println("syndication plugin: data is not PostRenderData!")
			return
		}
		parameterName := "syndication" // default
		if configParameterAny, ok := p.config["parameter"]; ok {
			if configParameter, ok := configParameterAny.(string); ok {
				parameterName = configParameter // override default from config
			}
		}
		syndicationLinks, ok := pd.GetPost().GetParameters()[parameterName]
		if !ok || len(syndicationLinks) == 0 {
			// No syndication links
			return
		}
		for _, link := range syndicationLinks {
			hb.WriteElementOpen("data", "value", link, "class", "u-syndication hide")
			hb.WriteElementClose("data")
		}
		return
	default:
		render(hb)
	}
}
