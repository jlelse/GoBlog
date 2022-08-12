package demoui

import (
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

func GetPlugin() plugintypes.UI {
	return &plugin{}
}

type plugin struct{}

func (*plugin) SetApp(_ plugintypes.App) {
	// Ignore
}

func (*plugin) SetConfig(_ map[string]any) {
	// Ignore
}

func (*plugin) Render(hb *htmlbuilder.HtmlBuilder, t plugintypes.RenderType, _ plugintypes.RenderData, render plugintypes.RenderNextFunc) {
	switch t {
	case plugintypes.PostMainElementRenderType:
		hb.WriteElementOpen("p")
		hb.WriteEscaped("Start of post main element")
		hb.WriteElementClose("p")
		render(hb)
		hb.WriteElementOpen("p")
		hb.WriteEscaped("End of post main element")
		hb.WriteElementClose("p")
		return
	default:
		render(hb)
	}
}
