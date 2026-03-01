package mcp

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"

	"go.goblog.app/app/pkgs/plugintypes"
)

const (
	protocolVersion = "2025-11-25"
	serverName      = "goblog-mcp"
	serverVersion   = "1.0.0"

	jsonRPCVersion = "2.0"

	errCodeParseError     = -32700
	errCodeInvalidRequest = -32600
	errCodeMethodNotFound = -32601
	errCodeInvalidParams  = -32602
)

type plugin struct {
	app  plugintypes.App
	path string
}

func GetPlugin() (
	plugintypes.SetApp,
	plugintypes.SetConfig,
	plugintypes.Middleware,
) {
	p := &plugin{}
	return p, p, p
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (p *plugin) SetConfig(config map[string]any) {
	p.path = "/mcp"
	if pa, ok := config["path"]; ok {
		if ps, ok := pa.(string); ok && ps != "" {
			p.path = ps
		}
	}
}

func (p *plugin) Prio() int {
	return 1000
}

func (p *plugin) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == p.path {
			p.handleMCP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// JSON-RPC types

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP types

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type capabilities struct {
	Tools *toolsCap `json:"tools,omitempty"`
}

type toolsCap struct {
	ListChanged bool `json:"listChanged"`
}

type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    capabilities `json:"capabilities"`
	ServerInfo      serverInfo   `json:"serverInfo"`
	Instructions    string       `json:"instructions,omitempty"`
}

type toolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []toolDef `json:"tools"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// Main handler

func (p *plugin) handleMCP(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept, MCP-Protocol-Version, MCP-Session-Id")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	switch r.Method {
	case http.MethodPost:
		p.handlePost(w, r)
	case http.MethodGet:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	case http.MethodDelete:
		w.WriteHeader(http.StatusOK)
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (p *plugin) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	// Check Basic Auth (app passwords) and session cookies
	if p.app.IsLoggedIn(r) {
		return true
	}
	// Check Bearer token as app password
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if p.app.CheckAppPassword(token) {
			return true
		}
	}
	w.Header().Set("WWW-Authenticate", `Bearer realm="mcp"`)
	w.WriteHeader(http.StatusUnauthorized)
	return false
}

func (p *plugin) handlePost(w http.ResponseWriter, r *http.Request) {
	if !p.checkAuth(w, r) {
		return
	}

	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, &jsonRPCResponse{
			JSONRPC: jsonRPCVersion,
			Error:   &rpcError{Code: errCodeParseError, Message: "Parse error"},
		})
		return
	}

	if req.JSONRPC != jsonRPCVersion {
		writeJSON(w, &jsonRPCResponse{
			JSONRPC: jsonRPCVersion,
			ID:      req.ID,
			Error:   &rpcError{Code: errCodeInvalidRequest, Message: "Invalid JSON-RPC version"},
		})
		return
	}

	// Handle notifications (no ID)
	if req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Handle requests
	var result any
	var rpcErr *rpcError

	switch req.Method {
	case "initialize":
		result = p.handleInitialize()
	case "ping":
		result = map[string]any{}
	case "tools/list":
		result = p.handleToolsList()
	case "tools/call":
		result, rpcErr = p.handleToolsCall(req.Params)
	default:
		rpcErr = &rpcError{Code: errCodeMethodNotFound, Message: "Method not found: " + req.Method}
	}

	resp := &jsonRPCResponse{
		JSONRPC: jsonRPCVersion,
		ID:      req.ID,
	}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}
	writeJSON(w, resp)
}

func (p *plugin) handleInitialize() *initializeResult {
	return &initializeResult{
		ProtocolVersion: protocolVersion,
		Capabilities: capabilities{
			Tools: &toolsCap{ListChanged: false},
		},
		ServerInfo: serverInfo{
			Name:    serverName,
			Version: serverVersion,
		},
		Instructions: "GoBlog MCP Server. Read-only access to blog posts. Use list_blogs to discover blogs, list_posts to browse posts, get_post to fetch a single post, search_posts for full-text search, and count_posts to count matching posts.",
	}
}

func (p *plugin) handleToolsList() *toolsListResult {
	return &toolsListResult{
		Tools: []toolDef{
			{
				Name:        "list_blogs",
				Description: "List available blogs with metadata such as title, description, and language.",
				InputSchema: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
				},
			},
			{
				Name:        "list_posts",
				Description: "List blog posts with optional filtering by blog, section, status, and visibility. Returns post metadata. Results are ordered by publication date descending.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"blog": map[string]any{
							"type":        "string",
							"description": "Filter by blog name",
						},
						"section": map[string]any{
							"type":        "string",
							"description": "Filter by section name",
						},
						"status": map[string]any{
							"type":        "string",
							"description": "Filter by post status (published, draft, scheduled). Default: published",
							"enum":        []string{"published", "draft", "scheduled"},
						},
						"visibility": map[string]any{
							"type":        "string",
							"description": "Filter by visibility (public, unlisted, private). Default: all",
							"enum":        []string{"public", "unlisted", "private"},
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Maximum number of posts to return (1-100). Default: 20",
							"minimum":     1,
							"maximum":     100,
						},
						"offset": map[string]any{
							"type":        "integer",
							"description": "Number of posts to skip for pagination. Default: 0",
							"minimum":     0,
						},
					},
					"additionalProperties": false,
				},
			},
			{
				Name:        "get_post",
				Description: "Get a single blog post by its path. Returns full post content and all metadata.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "The path of the post (e.g. /2024/01/my-post)",
						},
					},
					"required":             []string{"path"},
					"additionalProperties": false,
				},
			},
			{
				Name:        "search_posts",
				Description: "Full-text search across all blog posts. Returns matching posts with their metadata and content.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "The search query string",
						},
						"blog": map[string]any{
							"type":        "string",
							"description": "Filter by blog name",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Maximum number of results (1-100). Default: 20",
							"minimum":     1,
							"maximum":     100,
						},
						"offset": map[string]any{
							"type":        "integer",
							"description": "Number of results to skip for pagination. Default: 0",
							"minimum":     0,
						},
					},
					"required":             []string{"query"},
					"additionalProperties": false,
				},
			},
			{
				Name:        "count_posts",
				Description: "Count posts matching the given filters. Useful for understanding the size of the blog or specific sections.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"blog": map[string]any{
							"type":        "string",
							"description": "Filter by blog name",
						},
						"section": map[string]any{
							"type":        "string",
							"description": "Filter by section name",
						},
						"status": map[string]any{
							"type":        "string",
							"description": "Filter by post status (published, draft, scheduled). Default: published",
							"enum":        []string{"published", "draft", "scheduled"},
						},
					},
					"additionalProperties": false,
				},
			},
		},
	}
}

func (p *plugin) handleToolsCall(params json.RawMessage) (any, *rpcError) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, &rpcError{Code: errCodeInvalidParams, Message: "Invalid params: " + err.Error()}
	}

	switch call.Name {
	case "list_blogs":
		return p.toolListBlogs(call.Arguments), nil
	case "list_posts":
		return p.toolListPosts(call.Arguments), nil
	case "get_post":
		return p.toolGetPost(call.Arguments), nil
	case "search_posts":
		return p.toolSearchPosts(call.Arguments), nil
	case "count_posts":
		return p.toolCountPosts(call.Arguments), nil
	default:
		return nil, &rpcError{Code: errCodeInvalidParams, Message: "Unknown tool: " + call.Name}
	}
}

// Tool implementations

type blogResult struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Language    string `json:"language,omitempty"`
}

func (p *plugin) toolListBlogs(_ json.RawMessage) *toolCallResult {
	names := p.app.GetBlogNames()
	sort.Strings(names)
	results := make([]blogResult, 0, len(names))
	for _, name := range names {
		blog, ok := p.app.GetBlog(name)
		if !ok {
			log.Println("mcp: list_blogs missing blog:", name)
			return errorResult("Failed to list blogs")
		}
		results = append(results, blogResult{
			Name:        name,
			Title:       blog.GetTitle(),
			Description: blog.GetDescription(),
			Language:    blog.GetLanguage(),
		})
	}

	resultJSON, err := json.Marshal(results)
	if err != nil {
		log.Println("mcp: list_blogs error:", err)
		return errorResult("Failed to list blogs")
	}
	return &toolCallResult{
		Content: []textContent{{Type: "text", Text: string(resultJSON)}},
	}
}

type postResult struct {
	Path       string              `json:"path"`
	Title      string              `json:"title,omitempty"`
	Published  string              `json:"published,omitempty"`
	Updated    string              `json:"updated,omitempty"`
	Blog       string              `json:"blog"`
	Section    string              `json:"section,omitempty"`
	Status     string              `json:"status"`
	Visibility string              `json:"visibility"`
	Content    string              `json:"content,omitempty"`
	URL        string              `json:"url,omitempty"`
	Parameters map[string][]string `json:"parameters,omitempty"`
}

func (p *plugin) postToResult(post plugintypes.Post, includeContent bool) postResult {
	pr := postResult{
		Path:       post.GetPath(),
		Title:      post.GetTitle(),
		Published:  post.GetPublished(),
		Updated:    post.GetUpdated(),
		Blog:       post.GetBlog(),
		Section:    post.GetSection(),
		Status:     post.GetStatus(),
		Visibility: post.GetVisibility(),
		URL:        p.app.GetFullAddress(post.GetPath()),
	}
	if includeContent {
		pr.Content = post.GetContent()
		pr.Parameters = post.GetParameters()
	}
	return pr
}

func (p *plugin) toolListPosts(args json.RawMessage) *toolCallResult {
	var params struct {
		Blog       string `json:"blog"`
		Section    string `json:"section"`
		Status     string `json:"status"`
		Visibility string `json:"visibility"`
		Limit      int    `json:"limit"`
		Offset     int    `json:"offset"`
	}
	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult("Invalid arguments: " + err.Error())
		}
	}

	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}
	if params.Status == "" {
		params.Status = "published"
	}

	posts, err := p.app.GetPosts(&plugintypes.PostsQuery{
		Blog:       params.Blog,
		Section:    params.Section,
		Status:     params.Status,
		Visibility: params.Visibility,
		Limit:      params.Limit,
		Offset:     params.Offset,
	})
	if err != nil {
		log.Println("mcp: list_posts error:", err)
		return errorResult("Failed to list posts")
	}

	results := make([]postResult, len(posts))
	for i, post := range posts {
		results[i] = p.postToResult(post, false)
	}

	resultJSON, _ := json.Marshal(results)
	return &toolCallResult{
		Content: []textContent{{Type: "text", Text: string(resultJSON)}},
	}
}

func (p *plugin) toolGetPost(args json.RawMessage) *toolCallResult {
	var params struct {
		Path string `json:"path"`
	}
	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult("Invalid arguments: " + err.Error())
		}
	}
	if params.Path == "" {
		return errorResult("path is required")
	}

	post, err := p.app.GetPost(params.Path)
	if err != nil {
		return errorResult("Post not found: " + params.Path)
	}

	pr := p.postToResult(post, true)
	resultJSON, _ := json.Marshal(pr)
	return &toolCallResult{
		Content: []textContent{{Type: "text", Text: string(resultJSON)}},
	}
}

func (p *plugin) toolSearchPosts(args json.RawMessage) *toolCallResult {
	var params struct {
		Query  string `json:"query"`
		Blog   string `json:"blog"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult("Invalid arguments: " + err.Error())
		}
	}
	if params.Query == "" {
		return errorResult("query is required")
	}
	if params.Limit <= 0 || params.Limit > 100 {
		params.Limit = 20
	}

	posts, err := p.app.GetPosts(&plugintypes.PostsQuery{
		Search: params.Query,
		Blog:   params.Blog,
		Status: "published",
		Limit:  params.Limit,
		Offset: params.Offset,
	})
	if err != nil {
		log.Println("mcp: search_posts error:", err)
		return errorResult("Search failed")
	}

	results := make([]postResult, len(posts))
	for i, post := range posts {
		results[i] = p.postToResult(post, true)
	}

	resultJSON, _ := json.Marshal(results)
	return &toolCallResult{
		Content: []textContent{{Type: "text", Text: string(resultJSON)}},
	}
}

func (p *plugin) toolCountPosts(args json.RawMessage) *toolCallResult {
	var params struct {
		Blog    string `json:"blog"`
		Section string `json:"section"`
		Status  string `json:"status"`
	}
	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult("Invalid arguments: " + err.Error())
		}
	}
	if params.Status == "" {
		params.Status = "published"
	}

	count, err := p.app.CountPosts(&plugintypes.PostsQuery{
		Blog:    params.Blog,
		Section: params.Section,
		Status:  params.Status,
	})
	if err != nil {
		log.Println("mcp: count_posts error:", err)
		return errorResult("Failed to count posts")
	}

	resultJSON, _ := json.Marshal(map[string]int{"count": count})
	return &toolCallResult{
		Content: []textContent{{Type: "text", Text: string(resultJSON)}},
	}
}

// Helpers

func errorResult(msg string) *toolCallResult {
	return &toolCallResult{
		Content: []textContent{{Type: "text", Text: msg}},
		IsError: true,
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Println("mcp: error encoding response:", err)
	}
}
