package main

import (
	"crypto/rsa"
	"net/http"
	"sync"

	shutdowner "git.jlel.se/jlelse/go-shutdowner"
	ts "git.jlel.se/jlelse/template-strings"
	ct "github.com/elnormous/contenttype"
	"github.com/go-fed/httpsig"
	"github.com/hacdias/indieauth"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/yuin/goldmark"
	"go.goblog.app/app/pkgs/minify"
	"golang.org/x/sync/singleflight"
	"tailscale.com/tsnet"
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
	pPostHooks     []postHookFunc
	pUpdateHooks   []postHookFunc
	pDeleteHooks   []postHookFunc
	pUndeleteHooks []postHookFunc
	hourlyHooks    []hourlyHookFunc
	// HTTP Client
	httpClient *http.Client
	// HTTP Routers
	d http.Handler
	// IndexNow
	inKey  string
	inLoad singleflight.Group
	// IndieAuth
	ias *indieauth.Server
	// Logs
	logf *rotatelogs.RotateLogs
	// Markdown
	md, absoluteMd, titleMd goldmark.Markdown
	// Media
	compressorsInit  sync.Once
	compressors      []mediaCompression
	mediaStorageInit sync.Once
	mediaStorage     mediaStorage
	// Minify
	min minify.Minifier
	// Regex Redirects
	regexRedirects []*regexRedirect
	// Sessions
	loginSessions, captchaSessions *dbSessionStore
	// Shutdown
	shutdown shutdowner.Shutdowner
	// Template strings
	ts *ts.TemplateStrings
	// Tailscale
	tsinit sync.Once
	tss    *tsnet.Server
	// Tor
	torAddress  string
	torHostname string
}
