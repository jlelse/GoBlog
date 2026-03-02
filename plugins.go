package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"time"

	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/plugins"
	"go.goblog.app/app/pkgs/plugintypes"
	"go.goblog.app/app/pkgs/yaegiwrappers"
)

//go:embed plugins/*
var pluginsFS embed.FS

const (
	pluginSetAppType          = "setapp"
	pluginSetConfigType       = "setconfig"
	pluginUiType              = "ui"
	pluginUi2Type             = "ui2"
	pluginExecType            = "exec"
	pluginMiddlewareType      = "middleware"
	pluginUiSummaryType       = "uisummary"
	pluginUiPostType          = "uiPost"
	pluginUiPostContentType   = "uiPostContent"
	pluginUiFooterType        = "uifooter"
	pluginPostCreatedHookType = "postcreatedhook"
	pluginPostUpdatedHookType = "postupdatedhook"
	pluginPostDeletedHookType = "postdeletedhook"
)

func (a *goBlog) initPlugins() error {
	subFS, err := fs.Sub(pluginsFS, "plugins")
	if err != nil {
		return err
	}
	a.pluginHost = plugins.NewPluginHost(
		map[string]reflect.Type{
			pluginSetAppType:          reflect.TypeFor[plugintypes.SetApp](),
			pluginSetConfigType:       reflect.TypeFor[plugintypes.SetConfig](),
			pluginUiType:              reflect.TypeFor[plugintypes.UI](),
			pluginUi2Type:             reflect.TypeFor[plugintypes.UI2](),
			pluginExecType:            reflect.TypeFor[plugintypes.Exec](),
			pluginMiddlewareType:      reflect.TypeFor[plugintypes.Middleware](),
			pluginUiSummaryType:       reflect.TypeFor[plugintypes.UISummary](),
			pluginUiPostType:          reflect.TypeFor[plugintypes.UIPost](),
			pluginUiPostContentType:   reflect.TypeFor[plugintypes.UIPostContent](),
			pluginUiFooterType:        reflect.TypeFor[plugintypes.UIFooter](),
			pluginPostCreatedHookType: reflect.TypeFor[plugintypes.PostCreatedHook](),
			pluginPostUpdatedHookType: reflect.TypeFor[plugintypes.PostUpdatedHook](),
			pluginPostDeletedHookType: reflect.TypeFor[plugintypes.PostDeletedHook](),
		},
		yaegiwrappers.Symbols,
		subFS,
	)

	for _, pc := range a.cfg.Plugins {
		plugins, err := a.pluginHost.LoadPlugin(&plugins.PluginConfig{
			Path:       pc.Path,
			ImportPath: pc.Import,
		})
		if err != nil {
			return err
		}
		if p, ok := plugins[pluginSetConfigType]; ok {
			p.(plugintypes.SetConfig).SetConfig(pc.Config)
		}
		if p, ok := plugins[pluginSetAppType]; ok {
			p.(plugintypes.SetApp).SetApp(a)
		}
	}

	for _, p := range a.getPlugins(pluginExecType) {
		go p.(plugintypes.Exec).Exec()
	}

	return nil
}

func (a *goBlog) getPlugins(typ string) []any {
	if a.pluginHost == nil {
		return []any{}
	}
	return a.pluginHost.GetPlugins(typ)
}

// Implement all needed interfaces

func (a *goBlog) GetDatabase() plugintypes.Database {
	return a.db
}

func (a *goBlog) GetPost(path string) (plugintypes.Post, error) {
	return a.getPost(path)
}

func (a *goBlog) GetPosts(query *plugintypes.PostsQuery) ([]plugintypes.Post, error) {
	cfg := &postsRequestConfig{
		withoutRenderedTitle: true,
	}
	if query != nil {
		cfg.search = query.Search
		if query.Blog != "" {
			cfg.blogs = []string{query.Blog}
		}
		if query.Section != "" {
			cfg.sections = []string{query.Section}
		}
		if query.Status != "" {
			cfg.status = []postStatus{postStatus(query.Status)}
		}
		if query.Visibility != "" {
			cfg.visibility = []postVisibility{postVisibility(query.Visibility)}
		}
		cfg.parameter = query.Parameter
		cfg.parameterValue = query.ParameterValue
		cfg.limit = query.Limit
		cfg.offset = query.Offset
	}
	posts, err := a.getPosts(cfg)
	if err != nil {
		return nil, err
	}
	result := make([]plugintypes.Post, len(posts))
	for i, p := range posts {
		result[i] = p
	}
	return result, nil
}

func (a *goBlog) CountPosts(query *plugintypes.PostsQuery) (int, error) {
	cfg := &postsRequestConfig{}
	if query != nil {
		cfg.search = query.Search
		if query.Blog != "" {
			cfg.blogs = []string{query.Blog}
		}
		if query.Section != "" {
			cfg.sections = []string{query.Section}
		}
		if query.Status != "" {
			cfg.status = []postStatus{postStatus(query.Status)}
		}
		if query.Visibility != "" {
			cfg.visibility = []postVisibility{postVisibility(query.Visibility)}
		}
		cfg.parameter = query.Parameter
		cfg.parameterValue = query.ParameterValue
	}
	return a.db.countPosts(cfg)
}

func (a *goBlog) GetBlog(name string) (plugintypes.Blog, bool) {
	blog, ok := a.cfg.Blogs[name]
	return blog, ok
}

func (a *goBlog) GetBlogNames() []string {
	names := make([]string, 0, len(a.cfg.Blogs))
	for name := range a.cfg.Blogs {
		names = append(names, name)
	}
	return names
}

func (a *goBlog) PurgeCache() {
	a.purgeCache()
}

func (a *goBlog) GetHTTPClient() *http.Client {
	return a.httpClient
}

func (a *goBlog) CompileAsset(filename string, reader io.Reader) error {
	return a.compileAsset(filename, reader)
}

func (a *goBlog) AssetPath(filename string) string {
	return a.assetFileName(filename)
}

func (a *goBlog) SetPostParameter(path string, parameter string, values []string) error {
	return a.db.replacePostParam(path, parameter, values)
}

func (a *goBlog) CreatePost(content string) (plugintypes.Post, error) {
	p := &post{
		Content: content,
	}
	err := a.processContentAndParameters(p)
	if err != nil {
		return nil, err
	}
	err = a.createPost(p)
	if err != nil {
		return nil, err
	}
	return p, err
}

func (a *goBlog) UploadMedia(file io.Reader, filename string, _ string) (string, error) {
	recorder := httptest.NewRecorder()
	// Create a multipart form request
	body := bufferpool.Get()
	defer bufferpool.Put(body)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return "", err
	}
	err = writer.Close()
	if err != nil {
		return "", err
	}
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// Execute the request
	addAllScopes(a.getMicropubImplementation().getMediaHandler()).ServeHTTP(recorder, req)
	// Handle the recorder result
	res := recorder.Result()
	if recorder.Code < 200 || recorder.Code >= 400 {
		return "", fmt.Errorf("upload result: %s", res.Status)
	}
	// Extract the location header from the response
	return res.Header.Get("Location"), nil
}

func (a *goBlog) RenderMarkdownAsText(markdown string) (text string, err error) {
	return a.renderText(markdown)
}

func (a *goBlog) IsLoggedIn(req *http.Request) bool {
	return a.isLoggedIn(req)
}

func (a *goBlog) CheckAppPassword(password string) bool {
	valid, err := a.checkAppPassword(password)
	return err == nil && valid
}

func (a *goBlog) GetFullAddress(path string) string {
	return a.getFullAddress(path)
}

func (a *goBlog) GetSections(blog string) ([]plugintypes.Section, error) {
	sections, err := a.getSections(blog)
	if err != nil {
		return nil, err
	}
	result := make([]plugintypes.Section, 0, len(sections))
	for name, s := range sections {
		result = append(result, plugintypes.Section{
			Name:        name,
			Title:       s.Title,
			Description: s.Description,
		})
	}
	return result, nil
}

func (a *goBlog) GetTaxonomyValues(blog string, taxonomy string) ([]string, error) {
	return a.db.allTaxonomyValues(blog, taxonomy)
}

func (a *goBlog) GetComments(query *plugintypes.CommentsQuery) ([]plugintypes.Comment, error) {
	cfg := &commentsRequestConfig{}
	if query != nil {
		cfg.target = query.Target
		cfg.limit = query.Limit
		cfg.offset = query.Offset
	}
	comments, err := a.db.getComments(cfg)
	if err != nil {
		return nil, err
	}
	result := make([]plugintypes.Comment, 0, len(comments))
	for _, c := range comments {
		result = append(result, plugintypes.Comment{
			ID:      c.ID,
			Target:  c.Target,
			Name:    c.Name,
			Website: c.Website,
			Comment: c.Comment,
		})
	}
	return result, nil
}

func (a *goBlog) CountComments(query *plugintypes.CommentsQuery) (int, error) {
	cfg := &commentsRequestConfig{}
	if query != nil {
		cfg.target = query.Target
	}
	return a.db.countComments(cfg)
}

func (a *goBlog) GetWebmentions(query *plugintypes.WebmentionsQuery) ([]plugintypes.Webmention, error) {
	cfg := &webmentionsRequestConfig{}
	if query != nil {
		cfg.target = query.Target
		if query.Status != "" {
			cfg.status = webmentionStatus(query.Status)
		}
		cfg.limit = query.Limit
		cfg.offset = query.Offset
	}
	mentions, err := a.getWebmentions(cfg)
	if err != nil {
		return nil, err
	}
	result := make([]plugintypes.Webmention, 0, len(mentions))
	for _, m := range mentions {
		result = append(result, plugintypes.Webmention{
			Source:  m.Source,
			Target:  m.Target,
			Url:     m.Url,
			Created: time.Unix(m.Created, 0).Format(time.RFC3339),
			Title:   m.Title,
			Content: m.Content,
			Author:  m.Author,
			Status:  string(m.Status),
		})
	}
	return result, nil
}

func (a *goBlog) CountWebmentions(query *plugintypes.WebmentionsQuery) (int, error) {
	cfg := &webmentionsRequestConfig{}
	if query != nil {
		cfg.target = query.Target
		if query.Status != "" {
			cfg.status = webmentionStatus(query.Status)
		}
	}
	return a.db.countWebmentions(cfg)
}

func (a *goBlog) GetBlogStats(blog string) (*plugintypes.BlogStats, error) {
	if blog == "" {
		return nil, fmt.Errorf("blog name is required")
	}

	data, err := a.db.getBlogStats(blog)
	if err != nil {
		return nil, err
	}

	mapRow := func(r blogStatsRow) plugintypes.BlogStatsRow {
		return plugintypes.BlogStatsRow{
			Name:         r.Name,
			Posts:        r.Posts,
			Chars:        r.Chars,
			Words:        r.Words,
			WordsPerPost: r.WordsPerPost,
		}
	}

	res := &plugintypes.BlogStats{
		Total:  mapRow(data.Total),
		NoDate: mapRow(data.NoDate),
		Years:  make([]plugintypes.BlogStatsRow, len(data.Years)),
		Months: make(map[string][]plugintypes.BlogStatsRow, len(data.Months)),
	}

	for i, y := range data.Years {
		res.Years[i] = mapRow(y)
	}

	for k, m := range data.Months {
		res.Months[k] = make([]plugintypes.BlogStatsRow, len(m))
		for i, v := range m {
			res.Months[k][i] = mapRow(v)
		}
	}

	return res, nil
}

func (p *post) GetPath() string {
	return p.Path
}

func (p *post) GetParameters() map[string][]string {
	return p.Parameters
}

func (p *post) GetParameter(parameter string) []string {
	return p.Parameters[parameter]
}

func (p *post) GetFirstParameterValue(parameter string) string {
	return p.firstParameter(parameter)
}

func (p *post) GetSection() string {
	return p.Section
}

func (p *post) GetPublished() string {
	return p.Published
}

func (p *post) GetUpdated() string {
	return p.Updated
}

func (p *post) GetContent() string {
	return p.Content
}

func (p *post) GetTitle() string {
	return p.Title()
}

func (p *post) GetBlog() string {
	return p.Blog
}

func (p *post) GetStatus() string {
	return string(p.Status)
}

func (p *post) GetVisibility() string {
	return string(p.Visibility)
}

func (b *configBlog) GetLanguage() string {
	return b.Lang
}

func (b *configBlog) GetTitle() string {
	return b.Title
}

func (b *configBlog) GetDescription() string {
	return b.Description
}
