package main

import (
	"crypto/rsa"
	"net/http"
	"sync"

	shutdowner "git.jlel.se/jlelse/go-shutdowner"
	ts "git.jlel.se/jlelse/template-strings"
	"github.com/dgraph-io/ristretto"
	ct "github.com/elnormous/contenttype"
	apc "github.com/go-ap/client"
	"github.com/go-fed/httpsig"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/yuin/goldmark"
	"go.goblog.app/app/pkgs/minify"
	"go.goblog.app/app/pkgs/plugins"
	"go.hacdias.com/indielib/indieauth"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/sync/singleflight"
)

type goBlog struct {
	// ActivityPub
	apPrivateKey       *rsa.PrivateKey
	apPubKeyBytes      []byte
	apSigner           httpsig.Signer
	apSignMutex        sync.Mutex
	apHttpClients      map[string]*apc.C
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
	// Microformats
	mfInit  sync.Once
	mfCache *ristretto.Cache
	// Minify
	min minify.Minifier
	// Plugins
	pluginHost *plugins.PluginHost
	// Profile image
	profileImageHashString string
	profileImageHashGroup  singleflight.Group
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
	// Tor
	torAddress  string
	torHostname string
}
