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

	apikey, model string
}

func GetPlugin() (
	plugintypes.SetConfig, plugintypes.SetApp,
	plugintypes.PostCreatedHook,
	plugintypes.UIPost, plugintypes.UI2,
	plugintypes.Middleware,
) {
	p := &plugin{}
	return p, p, p, p, p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) SetConfig(config map[string]any) {
	p.config = config

	if k, ok := p.config["apikey"]; ok {
		if ks, ok := k.(string); ok {
			p.apikey = ks
		}
	}
	if m, ok := p.config["model"]; ok {
		if ms, ok := m.(string); ok {
			p.model = ms
		}
	}
}

func (p *plugin) PostCreated(post plugintypes.Post) {
	p.summarize(post)
}

func (p *plugin) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/x/aitldr" && p.app.IsLoggedIn(r) {
			if post, err := p.app.GetPost(r.FormValue("post")); err == nil {
				p.summarize(post)
				http.Redirect(w, r, post.GetPath(), http.StatusFound)
			} else {
				next.ServeHTTP(w, r)
			}
		} else if r.Method == http.MethodPost && r.URL.Path == "/x/aitldr/delete" && p.app.IsLoggedIn(r) {
			if post, err := p.app.GetPost(r.FormValue("post")); err == nil {
				p.deleteSummary(post)
				http.Redirect(w, r, post.GetPath(), http.StatusFound)
			} else {
				next.ServeHTTP(w, r)
			}
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

func (p *plugin) Prio() int {
	return 1000
}

const postParam = "aitldr"

func (p *plugin) RenderPost(renderContext plugintypes.RenderContext, post plugintypes.Post, doc *goquery.Document) {
	tldr := post.GetFirstParameterValue(postParam)

	// Add re-generation button
	if renderContext.IsLoggedIn() {
		buttonBuf := bufferpool.Get()
		defer bufferpool.Put(buttonBuf)
		buttonHw := htmlbuilder.NewHtmlBuilder(buttonBuf)
		buttonHw.WriteElementOpen("form", "method", "post", "action", "/x/aitldr")
		buttonHw.WriteElementOpen("input", "type", "hidden", "name", "post", "value", post.GetPath())
		buttonHw.WriteElementOpen("input", "type", "submit", "value", "(Re-)Generate AI summary")
		buttonHw.WriteElementClose("form")
		if tldr != "" {
			buttonHw.WriteElementOpen("form", "method", "post", "action", "/x/aitldr/delete")
			buttonHw.WriteElementOpen("input", "type", "hidden", "name", "post", "value", post.GetPath())
			buttonHw.WriteElementOpen("input", "type", "submit", "value", "Delete AI summary")
			buttonHw.WriteElementClose("form")
		}

		doc.Find("#posteditactions").AppendHtml(buttonBuf.String())
	}

	// If the post has a summary, display it
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

	if p.apikey == "" {
		log.Println("Config for aitldr plugin not correct! apikey missing!")
		return
	}

	prompt := p.createPrompt(post)
	if len(prompt) < 250 {
		log.Println("aitldr: Skip summarizing as post is too short", post.GetPath())
		return
	}

	var response apiResponse

	model := "gpt-4o"
	if p.model != "" {
		model = p.model
	}

	err := requests.URL("https://api.openai.com/v1/chat/completions").
		Method(http.MethodPost).
		Header("Authorization", "Bearer "+p.apikey).
		BodyJSON(map[string]any{
			"model": model,
			"messages": []apiMessage{
				{
					Role:    "system",
					Content: p.systemMessage(),
				},
				{
					Role:    "user",
					Content: prompt,
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

func (p *plugin) systemMessage() string {
	return `You are a summary writing plugin for a blogging platform.

Your task is to generate concise, first-person perspective summaries for lengthy blog posts, ensuring they capture the essence of the original content.

Guidelines:
1. Extract key points and present them in a clear, brief format.
2. Summaries must be in the same language as the blog post.
3. Limit the summary to a maximum of 250 characters, with no linebreaks.
4. Provide plain text without any formatting.
5. Write in the first person as if the author is summarizing their own post.
6. Avoid phrases like 'The author states' or 'The blogger argues.' Write as though the author is speaking directly.
7. Maintain the original intent and tone of the blog post.

Respond only with the summary content.`
}

func (p *plugin) createPrompt(post plugintypes.Post) string {
	prompt := ""
	if title, err := p.app.RenderMarkdownAsText(post.GetTitle()); err == nil && title != "" {
		prompt += title + "\n\n"
	} else if err != nil {
		log.Println("aitldr plugin: Rendering markdown as text failed:", err.Error())
	}
	if text, err := p.app.RenderMarkdownAsText(post.GetContent()); err == nil && text != "" {
		prompt += text
	} else if err != nil {
		log.Println("aitldr plugin: Rendering markdown as text failed:", err.Error())
	}
	return prompt
}

func (p *plugin) deleteSummary(post plugintypes.Post) {
	err := p.app.SetPostParameter(post.GetPath(), postParam, []string{})
	if err != nil {
		log.Println("aitldr plugin:", err.Error())
		return
	}

	p.app.PurgeCache()
}
