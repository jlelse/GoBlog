package customcss

import (
	"fmt"
	"os"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app plugintypes.App

	customCSS string
	init      sync.Once
	inited    bool
}

func GetPlugin() (plugintypes.SetConfig, plugintypes.SetApp, plugintypes.UI2) {
	p := &plugin{}
	return p, p, p
}

func (p *plugin) SetConfig(config map[string]any) {
	if filePath, ok := config["file"]; ok {
		if filePathString, ok := filePath.(string); ok {
			p.customCSS = filePathString
		}
	}
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) RenderWithDocument(_ plugintypes.RenderContext, doc *goquery.Document) {
	if p.app == nil || p.customCSS == "" {
		return
	}

	p.init.Do(func() {
		f, err := os.Open(p.customCSS)
		if err != nil {
			fmt.Println("Failed to open custom css file: ", err.Error())
			return
		}
		defer func() {
			_ = f.Close()
		}()

		err = p.app.CompileAsset("plugincustomcss.css", f)
		if err != nil {
			fmt.Println("Failed compile custom css: ", err.Error())
			return
		}

		p.inited = true
		fmt.Println("Custom CSS compiled")
	})

	if !p.inited {
		return
	}

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	hb := htmlbuilder.NewHtmlBuilder(buf)

	hb.WriteElementOpen("link", "rel", "stylesheet", "href", p.app.AssetPath("plugincustomcss.css"))

	doc.Find("head").AppendHtml(buf.String())
}
