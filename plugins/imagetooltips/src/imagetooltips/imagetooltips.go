package imagetooltips

import (
	"fmt"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app  plugintypes.App
	init sync.Once
}

func GetPlugin() (plugintypes.SetApp, plugintypes.UI2) {
	p := &plugin{}
	return p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) RenderWithDocument(_ plugintypes.RenderContext, doc *goquery.Document) {
	if p.app == nil {
		return
	}

	p.init.Do(func() {
		err := p.app.CompileAsset("imagetooltips.js", strings.NewReader(imagetooltipsJs))
		if err != nil {
			fmt.Println("Failed to compile js: ", err.Error())
			return
		}
	})

	bufJs := bufferpool.Get()
	defer bufferpool.Put(bufJs)
	hbJs := htmlbuilder.NewHtmlBuilder(bufJs)

	doc.Find(".e-content a > img").Each(func(_ int, element *goquery.Selection) {
		element.Parent().ReplaceWithSelection(element)
		element.AppendHtml("")
	})

	hbJs.WriteElementOpen("script", "src", p.app.AssetPath("imagetooltips.js"), "defer", "")
	hbJs.WriteElementClose("script")
	doc.Find("main").AppendHtml(bufJs.String())
}

// Copy as strings, as embedding is not supported by Yaegi

const imagetooltipsJs = `
(function () {
    document.querySelectorAll('.e-content img[title]').forEach((image) => {
        image.addEventListener('click', (e) => {
            e.preventDefault();
            alert(image.getAttribute('title'));
        });
    });
})();
`
