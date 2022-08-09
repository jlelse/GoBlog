package main

import (
	"crypto/rsa"
	"net/http"
	"sync"

	shutdowner "git.jlel.se/jlelse/go-shutdowner"
	ts "git.jlel.se/jlelse/template-strings"
	"github.com/dgraph-io/ristretto"
	ct "github.com/elnormous/contenttype"
	"github.com/go-fed/httpsig"
	"github.com/hacdias/indieauth/v2"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/yuin/goldmark"
	"go.goblog.app/app/pkgs/minify"
	"go.goblog.app/app/pkgs/plugins"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/sync/singleflight"
	"tailscale.com/tsnet"
)

type goBlog struct {
	// ActivityPub
	apPrivateKey       *rsa.PrivateKey
	apPubKeyBytes      []byte
	apPostSigner       httpsig.Signer
	apPostSignMutex    sync.Mutex
	webfingerResources map[string]*configBlog
	webfingerAccts     map[string]string
	// ActivityStreams
	asCheckMediaTypes []ct.MediaType
	// Assets
	assetFileNames map[string]string
	assetFiles     map[string]*assetFile
	// Autocert
	autocertManager *autocert.Manager
	autocertInit    sync.Once
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
	// Geo
	photonMutex sync.Mutex
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
	inKey  []byte
	inLoad sync.Once
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
	// Plugins
	pluginHost *plugins.PluginHost
	// Reactions
	reactionsInit  sync.Once
	reactionsCache *ristretto.Cache
	reactionsSfg   singleflight.Group
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
