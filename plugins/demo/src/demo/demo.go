package demo

import (
	"fmt"
	"io"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app    plugintypes.App
	config map[string]any
}

func GetPlugin() (
	plugintypes.SetApp, plugintypes.SetConfig,
	plugintypes.UI,
	plugintypes.Exec,
	plugintypes.Middleware,
) {
	p := &plugin{}
	return p, p, p, p, p
}

// SetApp
func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

// SetConfig
func (p *plugin) SetConfig(config map[string]any) {
	p.config = config
}

// UI
func (*plugin) Render(_ plugintypes.RenderContext, rendered io.Reader, modified io.Writer) {
	doc, err := goquery.NewDocumentFromReader(rendered)
	if err != nil {
		fmt.Println("demoui plugin: " + err.Error())
		return
	}
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	hb := htmlbuilder.NewHtmlBuilder(buf)
	hb.WriteElementOpen("p")
	hb.WriteEscaped("End of post content")
	hb.WriteElementClose("p")
	doc.Find("main.h-entry article div.e-content").AppendHtml(buf.String())
	_ = goquery.Render(modified, doc.Selection)
}

// Exec
func (p *plugin) Exec() {
	fmt.Println("Hello World from the demo plugin!")

	row, _ := p.app.GetDatabase().QueryRow("select count (*) from posts")
	var count int
	if err := row.Scan(&count); err != nil {
		fmt.Println(fmt.Errorf("failed to count posts: %w", err))
		return
	}

	fmt.Printf("Number of posts in database: %d", count)
	fmt.Println()
}

// Middleware
func (p *plugin) Prio() int {
	if prioAny, ok := p.config["prio"]; ok {
		if prio, ok := prioAny.(int); ok {
			return prio
		}
	}
	return 100
}

// Middleware
func (p *plugin) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Demo", fmt.Sprintf("This is from the demo middleware with prio %d", p.Prio()))
		next.ServeHTTP(w, r)
	})
}
