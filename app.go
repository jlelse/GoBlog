package main

import (
	"crypto/rsa"
	"html/template"
	"net/http"
	"sync"

	ts "git.jlel.se/jlelse/template-strings"
	"github.com/go-chi/chi/v5"
	"github.com/go-fed/httpsig"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/yuin/goldmark"
	"golang.org/x/sync/singleflight"
)

type goBlog struct {
	// ActivityPub
	apPrivateKey       *rsa.PrivateKey
	apPostSigner       httpsig.Signer
	apPostSignMutex    sync.Mutex
	webfingerResources map[string]*configBlog
	webfingerAccts     map[string]string
	// Assets
	assetFileNames map[string]string
	assetFiles     map[string]*assetFile
	// Blogroll
	blogrollCacheGroup singleflight.Group
	// Cache
	cache *cache
	// Config
	cfg *config
	// Database
	db *database
	// Hooks
	pPostHooks   []postHookFunc
	pUpdateHooks []postHookFunc
	pDeleteHooks []postHookFunc
	// HTTP
	d                      *dynamicHandler
	privateMode            bool
	privateModeHandler     []func(http.Handler) http.Handler
	captchaHandler         http.Handler
	micropubRouter         *chi.Mux
	indieAuthRouter        *chi.Mux
	webmentionsRouter      *chi.Mux
	notificationsRouter    *chi.Mux
	activitypubRouter      *chi.Mux
	editorRouter           *chi.Mux
	commentsRouter         *chi.Mux
	searchRouter           *chi.Mux
	setBlogMiddlewares     map[string]func(http.Handler) http.Handler
	sectionMiddlewares     map[string]func(http.Handler) http.Handler
	taxonomyMiddlewares    map[string]func(http.Handler) http.Handler
	photosMiddlewares      map[string]func(http.Handler) http.Handler
	searchMiddlewares      map[string]func(http.Handler) http.Handler
	customPagesMiddlewares map[string]func(http.Handler) http.Handler
	commentsMiddlewares    map[string]func(http.Handler) http.Handler
	// Logs
	logf *rotatelogs.RotateLogs
	// Markdown
	md, absoluteMd goldmark.Markdown
	// Regex Redirects
	regexRedirects []*regexRedirect
	// Rendering
	templates map[string]*template.Template
	// Sessions
	loginSessions, captchaSessions *dbSessionStore
	// Template strings
	ts *ts.TemplateStrings
	// Tor
	torAddress string
}
