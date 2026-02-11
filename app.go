package main

import (
	"crypto/rsa"
	"log/slog"
	"net/http"
	"sync"

	"code.superseriousbusiness.org/httpsig"
	shutdowner "git.jlel.se/jlelse/go-shutdowner"
	ts "git.jlel.se/jlelse/template-strings"
	ct "github.com/elnormous/contenttype"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/kaorimatz/go-opml"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	geojson "github.com/paulmach/go.geojson"
	"github.com/samber/go-singleflightx"
	"github.com/yuin/goldmark"
	c "go.goblog.app/app/pkgs/cache"
	"go.goblog.app/app/pkgs/minify"
	"go.goblog.app/app/pkgs/plugins"
	"go.hacdias.com/indielib/indieauth"
	"golang.org/x/crypto/acme/autocert"
)

type goBlog struct {
	// ActivityPub
	apPrivateKey       *rsa.PrivateKey
	apPubKeyBytes      []byte
	apSigner           httpsig.Signer
	apSignerNoDigest   httpsig.Signer
	apSignMutex        sync.Mutex
	webfingerResources map[string]*configBlog
	webfingerAccts     map[string]string
	apUserHandle       map[string]string
	// Assets
	assetFileNames map[string]string
	assetFiles     map[string]*assetFile
	// Autocert
	autocertManager *autocert.Manager
	autocertInit    sync.Once
	// Blogroll
	blogrollCacheGroup singleflightx.Group[string, []*opml.Outline]

	// Cache
	cache *cache
	// Config
	cfg *config
	// Database
	db *database
	// Errors
	errorCheckMediaTypes []ct.MediaType
	// Geo
	nominatimGroup singleflightx.Group[string, *geojson.FeatureCollection]
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
	// Inits
	initLogOnce, initMarkdownOnce, initSessionStoresOnce, initIndieAuthOnce, initCacheOnce sync.Once
	// Logs (HTTP)
	logf *rotatelogs.RotateLogs
	// Logs (Program)
	logger   *slog.Logger
	logLevel *slog.LevelVar
	// Markdown
	md, absoluteMd, titleMd goldmark.Markdown
	// Media
	compressorsInit  sync.Once
	compressors      []mediaCompression
	mediaStorageInit sync.Once
	mediaStorage     mediaStorage
	// Microformats
	mfInit  sync.Once
	mfCache *c.Cache[string, []byte]
	// Micropub
	mpImpl *micropubImplementation
	// Minify
	min minify.Minifier
	// Plugins
	pluginHost *plugins.PluginHost
	// Profile image
	profileImageHashString string
	profileImageHashGroup  *sync.Once
	// Reactions
	reactionsInit  sync.Once
	reactionsCache *c.Cache[string, string]
	reactionsSfg   singleflightx.Group[string, string]
	// Regex Redirects
	regexRedirects []*regexRedirect
	// Sessions
	loginSessions, captchaSessions, webauthnSessions *dbSessionStore
	// Shutdown
	shutdown shutdowner.Shutdowner
	// Template strings
	ts *ts.TemplateStrings
	// Tor
	torAddress  string
	torHostname string
	// WebAuthn
	webAuthn *webauthn.WebAuthn
}
