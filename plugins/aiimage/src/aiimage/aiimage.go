package aiimage

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/carlmjohnson/requests"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type plugin struct {
	app plugintypes.App

	config map[string]any

	apikey, model string
}

func GetPlugin() (
	plugintypes.SetConfig, plugintypes.SetApp,
	plugintypes.UIPostContent, plugintypes.UIPost,
	plugintypes.Middleware,
) {
	p := &plugin{}
	return p, p, p, p, p
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

func (p *plugin) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/x/aiimage" && p.app.IsLoggedIn(r) {
			if post, err := p.app.GetPost(r.FormValue("post")); err == nil {
				p.createCaptions(post)
				http.Redirect(w, r, post.GetPath(), http.StatusFound)
			} else {
				next.ServeHTTP(w, r)
			}
		} else if r.Method == http.MethodPost && r.URL.Path == "/x/aiimage/delete" && p.app.IsLoggedIn(r) {
			if post, err := p.app.GetPost(r.FormValue("post")); err == nil {
				p.deleteCaptions(post)
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

const postParam = "aiimage"

func (p *plugin) RenderPost(renderContext plugintypes.RenderContext, post plugintypes.Post, doc *goquery.Document) {
	if renderContext.IsLoggedIn() {
		buttonBuf := bufferpool.Get()
		defer bufferpool.Put(buttonBuf)
		buttonHw := htmlbuilder.NewHtmlBuilder(buttonBuf)
		buttonHw.WriteElementOpen("form", "method", "post", "action", "/x/aiimage")
		buttonHw.WriteElementOpen("input", "type", "hidden", "name", "post", "value", post.GetPath())
		buttonHw.WriteElementOpen("input", "type", "submit", "value", "(Re-)Generate AI image captions")
		buttonHw.WriteElementClose("form")
		if len(post.GetParameter(postParam)) != 0 {
			buttonHw.WriteElementOpen("form", "method", "post", "action", "/x/aiimage/delete")
			buttonHw.WriteElementOpen("input", "type", "hidden", "name", "post", "value", post.GetPath())
			buttonHw.WriteElementOpen("input", "type", "submit", "value", "Delete AI image captions")
			buttonHw.WriteElementClose("form")
		}
		doc.Find("#posteditactions").AppendHtml(buttonBuf.String())
	}
}

func (p *plugin) RenderPostContent(post plugintypes.Post, doc *goquery.Document) {
	captions := post.GetParameter(postParam)

	if len(captions) == 0 {
		return
	}

	title := "AI generated caption:"
	if blogConfig, ok := p.config[post.GetBlog()]; ok {
		if blogConfigAsMap, ok := blogConfig.(map[string]any); ok {
			if blogSpecificTitle, ok := blogConfigAsMap["title"]; ok {
				if blogSpecificTitleAsString, ok := blogSpecificTitle.(string); ok {
					title = blogSpecificTitleAsString
				}
			}
		}
	}

	for _, caption := range captions {
		captionSubstrings := strings.SplitN(caption, " : ", 2)
		if len(captionSubstrings) != 2 {
			continue
		}
		imageUrl, imageCaption := captionSubstrings[0], captionSubstrings[1]
		doc.Find(fmt.Sprintf(`img[src="%s"]`, imageUrl)).Each(func(_ int, element *goquery.Selection) {
			finalCaption := title + " " + imageCaption
			existingAlt, altExists := element.Attr("alt")
			if !altExists {
				element.SetAttr("alt", finalCaption)
			} else {
				element.SetAttr("alt", existingAlt+"\n\n"+finalCaption)
			}
			exisitingTitle, titleExists := element.Attr("title")
			if !titleExists {
				element.SetAttr("title", finalCaption)
			} else {
				element.SetAttr("title", exisitingTitle+"\n\n"+finalCaption)
			}
		})
	}
}

type apiRequestMessage struct {
	Role    string                      `json:"role"`
	Content []*apiRequestMessageContent `json:"content"`
}

type apiRequestMessageContent struct {
	Type     string                            `json:"type"`
	Text     string                            `json:"text,omitempty"`
	ImageUrl *apiRequestMessageContentImageUrl `json:"image_url,omitempty"`
}

type apiRequestMessageContentImageUrl struct {
	Url    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type apiResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type apiResponse struct {
	Choices []struct {
		Message apiResponseMessage `json:"message"`
	} `json:"choices"`
}

func (p *plugin) createCaptions(post plugintypes.Post) {
	if post.GetFirstParameterValue("noaiimage") == "true" {
		log.Println("aiimage: Skip summarizing", post.GetPath())
		return
	}

	images := post.GetParameter("images")
	if len(images) == 0 {
		return
	}

	if p.apikey == "" {
		log.Println("Config for aiimage plugin not correct! apikey missing!")
		return
	}

	model := "gpt-4o"
	if p.model != "" {
		model = p.model
	}

	blog, _ := p.app.GetBlog(post.GetBlog())
	postLang := blog.GetLanguage()

	captions := []string{}

	for _, image := range images {
		requestBody := map[string]any{
			"model": model,
			"messages": []apiRequestMessage{
				{
					Role: "system",
					Content: []*apiRequestMessageContent{{
						Type: "text",
						Text: p.systemMessage(postLang),
					}},
				},
				{
					Role: "user",
					Content: []*apiRequestMessageContent{{
						Type: "image_url",
						ImageUrl: &apiRequestMessageContentImageUrl{
							Url:    image,
							Detail: "low",
						},
					}},
				},
			},
		}

		var response apiResponse
		err := requests.URL("https://api.openai.com/v1/chat/completions").
			Method(http.MethodPost).
			Header("Authorization", "Bearer "+p.apikey).
			BodyJSON(requestBody).
			ToJSON(&response).
			Fetch(context.Background())

		if err != nil {
			log.Println("aiimage plugin:", err.Error())
			return
		}

		if len(response.Choices) < 1 {
			return
		}

		caption := response.Choices[0].Message.Content
		caption = strings.TrimSpace(caption)

		captions = append(captions, image+" : "+caption)
	}

	err := p.app.SetPostParameter(post.GetPath(), postParam, captions)
	if err != nil {
		log.Println("aiimage plugin:", err.Error())
		return
	}

	p.app.PurgeCache()
}

func (p *plugin) systemMessage(language string) string {
	return `Generate concise, factual captions (5-15 words) describing the main subject and action in an image.
Focus on clarity and accessibility for visually impaired users.
Avoid opinions, interpretations, or creative embellishments.
Generate the response in language '` + language + `'.`
}

func (p *plugin) deleteCaptions(post plugintypes.Post) {
	err := p.app.SetPostParameter(post.GetPath(), postParam, []string{})
	if err != nil {
		log.Println("aiimage plugin:", err.Error())
		return
	}

	p.app.PurgeCache()
}
