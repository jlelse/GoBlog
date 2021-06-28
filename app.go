package main

import (
	"crypto/rsa"
	"html/template"
	"net/http"
	"sync"

	shutdowner "git.jlel.se/jlelse/go-shutdowner"
	ts "git.jlel.se/jlelse/template-strings"
	ct "github.com/elnormous/contenttype"
	"github.com/go-chi/chi/v5"
	"github.com/go-fed/httpsig"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/yuin/goldmark"
	"go.goblog.app/app/pkgs/minify"
	"golang.org/x/sync/singleflight"
)

type goBlog struct {
	// ActivityPub
	apPrivateKey       *rsa.PrivateKey
	apPostSigner       httpsig.Signer
	apPostSignMutex    sync.Mutex
	webfingerResources map[string]*configBlog
	webfingerAccts     map[string]string
	// ActivityStreams
	asCheckMediaTypes []ct.MediaType
	// Assets
	assetFileNames map[string]string
	assetFiles     map[string]*assetFile
	// Blogroll
	blogrollCacheGroup singleflight.Group
	// Blogstats
	blogStatsCacheGroup singleflight.Group
	// Cache
	cache *cache
	// Config
	cfg *config
	// Database
	db *database
	// Errors
	errorCheckMediaTypes []ct.MediaType
	// Hooks
	pPostHooks   []postHookFunc
	pUpdateHooks []postHookFunc
	pDeleteHooks []postHookFunc
	hourlyHooks  []hourlyHookFunc
	// HTTP
	cspDomains string
	// HTTP Client
	httpClient httpClient
	// HTTP Routers
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
	taxValueMiddlewares    map[string]func(http.Handler) http.Handler
	photosMiddlewares      map[string]func(http.Handler) http.Handler
	searchMiddlewares      map[string]func(http.Handler) http.Handler
	customPagesMiddlewares map[string]func(http.Handler) http.Handler
	commentsMiddlewares    map[string]func(http.Handler) http.Handler
	// Logs
	logf *rotatelogs.RotateLogs
	// Markdown
	md, absoluteMd goldmark.Markdown
	// Media
	compressorsInit  sync.Once
	compressors      []mediaCompression
	mediaStorageInit sync.Once
	mediaStorage     mediaStorage
	// Minify
	min minify.Minifier
	// Regex Redirects
	regexRedirects []*regexRedirect
	// Rendering
	templates map[string]*template.Template
	// Sessions
	loginSessions, captchaSessions *dbSessionStore
	// Shutdown
	shutdown shutdowner.Shutdowner
	// Template strings
	ts *ts.TemplateStrings
	// Tor
	torAddress string
}
