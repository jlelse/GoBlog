package aitldr

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/carlmjohnson/requests"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app plugintypes.App

	config  map[string]any
	initCSS sync.Once
}

func GetPlugin() (
	plugintypes.SetConfig, plugintypes.SetApp,
	plugintypes.PostCreatedHook, plugintypes.PostUpdatedHook,
	plugintypes.UIPost, plugintypes.UI2,
) {
	p := &plugin{}
	return p, p, p, p, p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) SetConfig(config map[string]any) {
	p.config = config
}

func (p *plugin) PostCreated(post plugintypes.Post) {
	p.summarize(post)
}

func (p *plugin) PostUpdated(post plugintypes.Post) {
	p.summarize(post)
}

const postParam = "aitldr"

func (p *plugin) RenderPost(renderContext plugintypes.RenderContext, post plugintypes.Post, doc *goquery.Document) {
	tldr := post.GetFirstParameterValue(postParam)
	if tldr == "" {
		return
	}

	title := "AI generated summary:"
	if blogConfig, ok := p.config[renderContext.GetBlog()]; ok {
		if blogConfigAsMap, ok := blogConfig.(map[string]any); ok {
			if blogSpecificTitle, ok := blogConfigAsMap["title"]; ok {
				if blogSpecificTitleAsString, ok := blogSpecificTitle.(string); ok {
					title = blogSpecificTitleAsString
				}
			}
		}
	}

	buf := bufferpool.Get()
	defer bufferpool.Put(buf)
	hw := htmlbuilder.NewHtmlBuilder(buf)
	hw.WriteElementOpen("div", "class", "p aitldr")
	hw.WriteElementOpen("b")
	hw.WriteEscaped(title)
	hw.WriteElementClose("b")
	hw.WriteEscaped(" ")
	hw.WriteElementOpen("i")
	hw.WriteEscaped(tldr)
	hw.WriteElementsClose("i", "div")

	doc.Find(".h-entry > article > .e-content").BeforeHtml(buf.String())
}

const customCSS = ".aitldr { border: 1px dashed; padding: 1em; }"

func (p *plugin) RenderWithDocument(_ plugintypes.RenderContext, doc *goquery.Document) {
	if p.app == nil {
		return
	}

	// Init custom CSS for plugin
	p.initCSS.Do(func() {
		_ = p.app.CompileAsset("aitldr.css", strings.NewReader(customCSS))
	})

	// Check if page has AI TLDR, then add the custom CSS
	doc.Find(".aitldr").First().Each(func(_ int, _ *goquery.Selection) {
		buf := bufferpool.Get()
		defer bufferpool.Put(buf)
		hb := htmlbuilder.NewHtmlBuilder(buf)
		hb.WriteElementOpen("link", "rel", "stylesheet", "href", p.app.AssetPath("aitldr.css"))
		doc.Find("head").AppendHtml(buf.String())
	})
}

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiResponse struct {
	Choices []struct {
		Message apiMessage `json:"message"`
	} `json:"choices"`
}

func (p *plugin) summarize(post plugintypes.Post) {
	if post.GetFirstParameterValue("noaitldr") == "true" {
		log.Println("aitldr: Skip summarizing", post.GetPath())
		return
	}

	apikey := ""
	if k, ok := p.config["apikey"]; ok {
		if ks, ok := k.(string); ok {
			apikey = ks
		}
	}
	if apikey == "" {
		log.Println("Config for aitldr plugin not correct! apikey missing!")
		return
	}

	var response apiResponse

	err := requests.URL("https://api.openai.com/v1/chat/completions").
		Method(http.MethodPost).
		Header("Authorization", "Bearer "+apikey).
		BodyJSON(map[string]any{
			"model":       "gpt-3.5-turbo",
			"temperature": 0,
			"max_tokens":  200,
			"messages": []apiMessage{
				{
					Role:    "user",
					Content: p.createPrompt(post),
				},
			},
		}).
		ToJSON(&response).
		Fetch(context.Background())

	if err != nil {
		log.Println("aitldr plugin:", err.Error())
		return
	}

	if len(response.Choices) < 1 {
		return
	}

	summary := response.Choices[0].Message.Content
	summary = strings.TrimSpace(summary)

	err = p.app.SetPostParameter(post.GetPath(), postParam, []string{summary})
	if err != nil {
		log.Println("aitldr plugin:", err.Error())
		return
	}

	p.app.PurgeCache()
}

func (p *plugin) createPrompt(post plugintypes.Post) string {
	lang := "en"
	if blog, ok := p.app.GetBlog(post.GetBlog()); ok {
		if blogLang := blog.GetLanguage(); lang != "" {
			lang = blogLang
		}
	}
	prompt := "Summarize the content of following blog post in one sentence in the language \"" + lang + "\". Answer with just the summary.\n\n\n"
	if title, err := p.app.RenderMarkdownAsText(post.GetTitle()); err == nil && title != "" {
		prompt += title + "\n\n"
	} else if err != nil {
		log.Println("aitldr plugin: Rendering markdown as text failed:", err.Error())
	}
	if text, err := p.app.RenderMarkdownAsText(post.GetContent()); err == nil && text != "" {
		prompt += text + "\n\n"
	} else if err != nil {
		log.Println("aitldr plugin: Rendering markdown as text failed:", err.Error())
	}
	return prompt
}
