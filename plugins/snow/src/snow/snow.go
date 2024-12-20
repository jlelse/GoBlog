package snow

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
		err := p.app.CompileAsset("snow.css", strings.NewReader(snowCss))
		if err != nil {
			fmt.Println("Failed compile snow css: ", err.Error())
			return
		}
		err = p.app.CompileAsset("snow.js", strings.NewReader(snowJs))
		if err != nil {
			fmt.Println("Failed compile snow js: ", err.Error())
			return
		}
	})

	bufCss, bufJs := bufferpool.Get(), bufferpool.Get()
	defer bufferpool.Put(bufCss, bufJs)
	hbCss, hbJs := htmlbuilder.NewHtmlBuilder(bufCss), htmlbuilder.NewHtmlBuilder(bufJs)

	hbCss.WriteElementOpen("link", "rel", "stylesheet", "href", p.app.AssetPath("snow.css"))
	doc.Find("head").AppendHtml(bufCss.String())

	hbJs.WriteElementOpen("script", "src", p.app.AssetPath("snow.js"), "defer", "")
	hbJs.WriteElementClose("script")
	doc.Find("main").AppendHtml(bufJs.String())
}

// Copy as strings, as embedding is not supported by Yaegi

const snowCss = `
.snowflake {
    position: absolute;
    top: -10px;
    font-size: 1.5em;
    pointer-events: none;
    animation-name: fall;
    animation-timing-function: linear;
    animation-iteration-count: 1;
}

@keyframes fall {
    0% {
        transform: translateY(-10px) translateX(0);
        opacity: 1;
    }

    100% {
        transform: translateY(95vh) translateX(0);
        opacity: 0;
    }
}
`

const snowJs = `
(function () {
    function createSnowflake() {
        const snowflake = document.createElement('div');
        snowflake.classList.add('snowflake');
        snowflake.style.left = Math.random() * 100 + 'vw';
        snowflake.style.animationDuration = Math.random() * 10 + 5 + 's';
        snowflake.innerText = '❄';
        document.body.appendChild(snowflake);
        snowflake.addEventListener('animationend', () => {
            snowflake.remove();
        });
    }
    setInterval(createSnowflake, 200);
})()
`
