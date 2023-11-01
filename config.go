package main

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/samber/lo"
	"github.com/spf13/viper"
	"maunium.net/go/mautrix"
)

type config struct {
	Server        *configServer          `mapstructure:"server"`
	Db            *configDb              `mapstructure:"database"`
	Cache         *configCache           `mapstructure:"cache"`
	DefaultBlog   string                 `mapstructure:"defaultblog"`
	Blogs         map[string]*configBlog `mapstructure:"blogs"`
	User          *configUser            `mapstructure:"user"`
	Hooks         *configHooks           `mapstructure:"hooks"`
	Plugins       []*configPlugin        `mapstructure:"plugins"`
	Micropub      *configMicropub        `mapstructure:"micropub"`
	PathRedirects []*configRegexRedirect `mapstructure:"pathRedirects"`
	ActivityPub   *configActivityPub     `mapstructure:"activityPub"`
	Webmention    *configWebmention      `mapstructure:"webmention"`
	Notifications *configNotifications   `mapstructure:"notifications"`
	PrivateMode   *configPrivateMode     `mapstructure:"privateMode"`
	IndexNow      *configIndexNow        `mapstructure:"indexNow"`
	EasterEgg     *configEasterEgg       `mapstructure:"easterEgg"`
	MapTiles      *configMapTiles        `mapstructure:"mapTiles"`
	TTS           *configTTS             `mapstructure:"tts"`
	Reactions     *configReactions       `mapstructure:"reactions"`
	Pprof         *configPprof           `mapstructure:"pprof"`
	RobotsTxt     *configRobotsTxt       `mapstructure:"robotstxt"`
	Debug         bool                   `mapstructure:"debug"`
	initialized   bool
}

type configServer struct {
	Logging             bool     `mapstructure:"logging"`
	LogFile             string   `mapstructure:"logFile"`
	Port                int      `mapstructure:"port"`
	PublicAddress       string   `mapstructure:"publicAddress"`
	ShortPublicAddress  string   `mapstructure:"shortPublicAddress"`
	MediaAddress        string   `mapstructure:"mediaAddress"`
	PublicHTTPS         bool     `mapstructure:"publicHttps"`
	AcmeDir             string   `mapstructure:"acmeDir"`
	AcmeEabKid          string   `mapstructure:"acmeEabKid"`
	AcmeEabKey          string   `mapstructure:"acmeEabKey"`
	HttpsCert           string   `mapstructure:"httpsCert"`
	HttpsKey            string   `mapstructure:"httpsKey"`
	HttpsRedirect       bool     `mapstructure:"httpsRedirect"`
	Tor                 bool     `mapstructure:"tor"`
	TorSingleHop        bool     `mapstructure:"torSingleHop"`
	SecurityHeaders     bool     `mapstructure:"securityHeaders"`
	CSPDomains          []string `mapstructure:"cspDomains"`
	publicHostname      string
	shortPublicHostname string
	mediaHostname       string
	manualHttps         bool
}

type configDb struct {
	File     string `mapstructure:"file"`
	DumpFile string `mapstructure:"dumpFile"`
	Debug    bool   `mapstructure:"debug"`
}

type configCache struct {
	Enable     bool `mapstructure:"enable"`
	Expiration int  `mapstructure:"expiration"`
}

type configBlog struct {
	Path           string                    `mapstructure:"path"`
	Lang           string                    `mapstructure:"lang"`
	Title          string                    `mapstructure:"title"`
	Description    string                    `mapstructure:"description"`
	Pagination     int                       `mapstructure:"pagination"`
	DefaultSection string                    `mapstructure:"defaultsection"`
	Sections       map[string]*configSection `mapstructure:"sections"`
	Taxonomies     []*configTaxonomy         `mapstructure:"taxonomies"`
	Menus          map[string]*configMenu    `mapstructure:"menus"`
	Photos         *configPhotos             `mapstructure:"photos"`
	Search         *configSearch             `mapstructure:"search"`
	BlogStats      *configBlogStats          `mapstructure:"blogStats"`
	Blogroll       *configBlogroll           `mapstructure:"blogroll"`
	Telegram       *configTelegram           `mapstructure:"telegram"`
	PostAsHome     bool                      `mapstructure:"postAsHome"`
	RandomPost     *configRandomPost         `mapstructure:"randomPost"`
	OnThisDay      *configOnThisDay          `mapstructure:"onThisDay"`
	Comments       *configComments           `mapstructure:"comments"`
	Map            *configGeoMap             `mapstructure:"map"`
	Contact        *configContact            `mapstructure:"contact"`
	Announcement   *configAnnouncement       `mapstructure:"announcement"`
	// Configs read from database
	hideOldContentWarning bool
	hideShareButton       bool
	hideTranslateButton   bool
	addReplyTitle         bool
	addReplyContext       bool
	addLikeTitle          bool
	addLikeContext        bool
	// Editor state WebSockets
	esws sync.Map
	esm  sync.Mutex
}

type configSection struct {
	Title        string `mapstructure:"title"`
	Description  string `mapstructure:"description"`
	PathTemplate string `mapstructure:"pathtemplate"`
	ShowFull     bool   `mapstructure:"showFull"`
	HideOnStart  bool   `mapstructure:"hideOnStart"`
	Name         string
}

type configTaxonomy struct {
	Name        string `mapstructure:"name"`
	Title       string `mapstructure:"title"`
	Description string `mapstructure:"description"`
}

type configMenu struct {
	Items []*configMenuItem `mapstructure:"items"`
}

type configMenuItem struct {
	Title string `mapstructure:"title"`
	Link  string `mapstructure:"link"`
}

type configPhotos struct {
	Enabled     bool   `mapstructure:"enabled"`
	Path        string `mapstructure:"path"`
	Title       string `mapstructure:"title"`
	Description string `mapstructure:"description"`
}

type configSearch struct {
	Enabled     bool   `mapstructure:"enabled"`
	Path        string `mapstructure:"path"`
	Title       string `mapstructure:"title"`
	Description string `mapstructure:"description"`
	Placeholder string `mapstructure:"placeholder"`
}

type configBlogStats struct {
	Enabled     bool   `mapstructure:"enabled"`
	Path        string `mapstructure:"path"`
	Title       string `mapstructure:"title"`
	Description string `mapstructure:"description"`
}

type configBlogroll struct {
	Enabled     bool     `mapstructure:"enabled"`
	Path        string   `mapstructure:"path"`
	Opml        string   `mapstructure:"opml"`
	AuthHeader  string   `mapstructure:"authHeader"`
	AuthValue   string   `mapstructure:"authValue"`
	Categories  []string `mapstructure:"categories"`
	Title       string   `mapstructure:"title"`
	Description string   `mapstructure:"description"`
}

type configRandomPost struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type configOnThisDay struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type configComments struct {
	Enabled bool `mapstructure:"enabled"`
}

type configGeoMap struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type configContact struct {
	Enabled       bool   `mapstructure:"enabled"`
	Path          string `mapstructure:"path"`
	Title         string `mapstructure:"title"`
	Description   string `mapstructure:"description"`
	PrivacyPolicy string `mapstructure:"privacyPolicy"`
	SMTPHost      string `mapstructure:"smtpHost"`
	SMTPPort      int    `mapstructure:"smtpPort"`
	SMTPUser      string `mapstructure:"smtpUser"`
	SMTPPassword  string `mapstructure:"smtpPassword"`
	EmailFrom     string `mapstructure:"emailFrom"`
	EmailTo       string `mapstructure:"emailTo"`
	EmailSubject  string `mapstructure:"emailSubject"`
}

type configAnnouncement struct {
	Text string `mapstructure:"text"`
}

type configUser struct {
	Nick         string               `mapstructure:"nick"`
	Name         string               `mapstructure:"name"`
	Password     string               `mapstructure:"password"`
	TOTP         string               `mapstructure:"totp"`
	AppPasswords []*configAppPassword `mapstructure:"appPasswords"`
	Email        string               `mapstructure:"email"`
	Link         string               `mapstructure:"link"`
	Identities   []string             `mapstructure:"identities"`
}

type configAppPassword struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type configHooks struct {
	Shell    string   `mapstructure:"shell"`
	Hourly   []string `mapstructure:"hourly"`
	PreStart []string `mapstructure:"prestart"`
	// Can use template
	PostPost     []string `mapstructure:"postpost"`
	PostUpdate   []string `mapstructure:"postupdate"`
	PostDelete   []string `mapstructure:"postdelete"`
	PostUndelete []string `mapstructure:"postundelete"`
}

type configMicropub struct {
	CategoryParam         string               `mapstructure:"categoryParam"`
	ReplyParam            string               `mapstructure:"replyParam"`
	ReplyTitleParam       string               `mapstructure:"replyTitleParam"`
	ReplyContextParam     string               `mapstructure:"replyContextParam"`
	LikeParam             string               `mapstructure:"likeParam"`
	LikeTitleParam        string               `mapstructure:"likeTitleParam"`
	LikeContextParam      string               `mapstructure:"likeContextParam"`
	BookmarkParam         string               `mapstructure:"bookmarkParam"`
	AudioParam            string               `mapstructure:"audioParam"`
	PhotoParam            string               `mapstructure:"photoParam"`
	PhotoDescriptionParam string               `mapstructure:"photoDescriptionParam"`
	LocationParam         string               `mapstructure:"locationParam"`
	MediaStorage          *configMicropubMedia `mapstructure:"mediaStorage"`
}

type configMicropubMedia struct {
	MediaURL string `mapstructure:"mediaUrl"`
	// BunnyCDN
	BunnyStorageKey    string `mapstructure:"bunnyStorageKey"`
	BunnyStorageName   string `mapstructure:"bunnyStorageName"`
	BunnyStorageRegion string `mapstructure:"bunnyStorageRegion"`
	// FTP
	FTPAddress  string `mapstructure:"ftpAddress"`
	FTPUser     string `mapstructure:"ftpUser"`
	FTPPassword string `mapstructure:"ftpPassword"`
	// Tinify
	TinifyKey string `mapstructure:"tinifyKey"`
	// Cloudflare
	CloudflareCompressionEnabled bool `mapstructure:"cloudflareCompressionEnabled"`
	// Local
	LocalCompressionEnabled bool `mapstructure:"localCompressionEnabled"`
}

type configRegexRedirect struct {
	From string `mapstructure:"from"`
	To   string `mapstructure:"to"`
	Type int    `mapstructure:"type"`
}

type configActivityPub struct {
	Enabled        bool     `mapstructure:"enabled"`
	TagsTaxonomies []string `mapstructure:"tagsTaxonomies"`
}

type configNotifications struct {
	Ntfy     *configNtfy     `mapstructure:"ntfy"`
	Telegram *configTelegram `mapstructure:"telegram"`
	Matrix   *configMatrix   `mapstructure:"matrix"`
}

type configNtfy struct {
	Enabled bool   `mapstructure:"enabled"`
	Topic   string `mapstructure:"topic"`
	Server  string `mapstructure:"server"`
	User    string `mapstructure:"user"`
	Pass    string `mapstructure:"pass"`
	Email   string `mapstructure:"email"`
}

type configTelegram struct {
	Enabled         bool   `mapstructure:"enabled"`
	ChatID          string `mapstructure:"chatId"`
	BotToken        string `mapstructure:"botToken"`
	InstantViewHash string `mapstructure:"instantViewHash"`
}

type configMatrix struct {
	Enabled    bool   `mapstructure:"enabled"`
	HomeServer string `mapstructure:"homeserver"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	Room       string `mapstructure:"room"`
	DeviceId   string `mapstructure:"deviceid"`
	client     *mautrix.Client
	err        error
	clientInit sync.Once
}

type configPrivateMode struct {
	Enabled bool `mapstructure:"enabled"`
}

type configIndexNow struct {
	Enabled bool `mapstructure:"enabled"`
}

type configEasterEgg struct {
	Enabled bool `mapstructure:"enabled"`
}

type configWebmention struct {
	DisableSending   bool `mapstructure:"disableSending"`
	DisableReceiving bool `mapstructure:"disableReceiving"`
}

type configMapTiles struct {
	Source      string `mapstructure:"source"`
	Attribution string `mapstructure:"attribution"`
	MinZoom     int    `mapstructure:"minZoom"`
	MaxZoom     int    `mapstructure:"maxZoom"`
}

type configTTS struct {
	Enabled      bool   `mapstructure:"enabled"`
	GoogleAPIKey string `mapstructure:"googleApiKey"`
}

type configReactions struct {
	Enabled bool `mapstructure:"enabled"`
}

type configPprof struct {
	Enabled bool   `mapstructure:"enabled"`
	Address string `mapstructure:"address"`
}

type configPlugin struct {
	Path   string         `mapstructure:"path"`
	Import string         `mapstructure:"import"`
	Config map[string]any `mapstructure:"config"`
}

type configRobotsTxt struct {
	BlockedBots []string `mapstructure:"blockedBots"`
}

func (a *goBlog) loadConfigFile(file string) error {
	// Use viper to load the config file
	v := viper.New()
	if file != "" {
		// Use config file from the flag
		v.SetConfigFile(file)
	} else {
		// Search in default locations
		v.SetConfigName("config")
		v.AddConfigPath("./config/")
	}
	// Read config
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	// Unmarshal config
	a.cfg = createDefaultConfig()
	return v.Unmarshal(a.cfg)
}

func (a *goBlog) initConfig(logging bool) error {
	if a.cfg == nil {
		a.cfg = createDefaultConfig()
	}
	if a.cfg.initialized {
		return nil
	}
	// Init database
	if err := a.initDatabase(logging); err != nil {
		return err
	}
	// Parse addresses and hostnames
	if a.cfg.Server.PublicAddress == "" {
		return errors.New("no public address configured")
	}
	publicURL, err := url.Parse(a.cfg.Server.PublicAddress)
	if err != nil {
		return errors.New("Invalid public address: " + err.Error())
	}
	a.cfg.Server.publicHostname = publicURL.Hostname()
	if sa := a.cfg.Server.ShortPublicAddress; sa != "" {
		shortPublicURL, err := url.Parse(sa)
		if err != nil {
			return errors.New("Invalid short public address: " + err.Error())
		}
		a.cfg.Server.shortPublicHostname = shortPublicURL.Hostname()
	}
	if ma := a.cfg.Server.MediaAddress; ma != "" {
		mediaUrl, err := url.Parse(ma)
		if err != nil {
			return errors.New("Invalid media address: " + err.Error())
		}
		a.cfg.Server.mediaHostname = mediaUrl.Hostname()
	}
	// Check port or set default
	if a.cfg.Server.Port == 0 {
		finalPort := 8080
		if port := publicURL.Port(); port != "" {
			finalPort, err = strconv.Atoi(port)
			if err != nil {
				return errors.New("Failed to parse port: " + err.Error())
			}
		} else if publicURL.Scheme == "https" {
			finalPort = 443
		}
		a.cfg.Server.Port = finalPort
	}
	// Check HTTPS
	if a.cfg.Server.HttpsCert != "" && a.cfg.Server.HttpsKey != "" {
		a.cfg.Server.manualHttps = true
	}
	if a.cfg.Server.PublicHTTPS || a.cfg.Server.manualHttps {
		a.cfg.Server.SecurityHeaders = true
	}
	if a.cfg.Server.PublicHTTPS {
		a.cfg.Server.HttpsRedirect = true
	}
	// Check if any blog is configured
	if a.cfg.Blogs == nil || len(a.cfg.Blogs) == 0 {
		a.cfg.Blogs = map[string]*configBlog{
			"default": createDefaultBlog(),
		}
	}
	// Check if default blog is set
	if a.cfg.DefaultBlog == "" {
		// Set default blog to the only blog that is configured
		a.cfg.DefaultBlog = lo.Keys(a.cfg.Blogs)[0]
	}
	// Check if default blog exists
	if a.cfg.Blogs[a.cfg.DefaultBlog] == nil {
		return errors.New("default blog does not exist")
	}
	// Check media storage config
	if ms := a.cfg.Micropub.MediaStorage; ms != nil && ms.MediaURL != "" {
		ms.MediaURL = strings.TrimSuffix(ms.MediaURL, "/")
	}
	// Check if webmention receiving is disabled
	if wm := a.cfg.Webmention; wm != nil && wm.DisableReceiving {
		// Disable comments for all blogs
		for _, b := range a.cfg.Blogs {
			b.Comments = &configComments{Enabled: false}
		}
	}
	// Check if sections already migrated to db
	const sectionMigrationKey = "sections_migrated"
	if val, err := a.getSettingValue(sectionMigrationKey); err != nil {
		return err
	} else if val == "" {
		if err = a.saveAllSections(); err != nil {
			return err
		}
		if err = a.saveSettingValue(sectionMigrationKey, "1"); err != nil {
			return err
		}
	}
	// Load db sections
	if err = a.loadSections(); err != nil {
		return err
	}
	// Load other settings from database
	// User nick
	if userNick, err := a.getSettingValue(userNickSetting); err != nil {
		return err
	} else if userNick == "" {
		// Migrate to database
		if err = a.saveSettingValue(userNickSetting, a.cfg.User.Nick); err != nil {
			return err
		}
	} else {
		a.cfg.User.Nick = userNick
	}
	// User name
	if userName, err := a.getSettingValue(userNameSetting); err != nil {
		return err
	} else if userName == "" {
		// Migrate to database
		if err = a.saveSettingValue(userNameSetting, a.cfg.User.Name); err != nil {
			return err
		}
	} else {
		a.cfg.User.Name = userName
	}
	// Check config for each blog
	for blog, bc := range a.cfg.Blogs {
		// Check pagination
		if bc.Pagination == 0 {
			bc.Pagination = 10
		}
		// Check sections and add section if none exists
		if len(bc.Sections) == 0 {
			bc.Sections = createDefaultSections()
			if err = a.saveAllSections(); err != nil {
				return err
			}
		}
		// Check default section
		if defaultSection, err := a.getSettingValue(settingNameWithBlog(blog, defaultSectionSetting)); err != nil {
			// Failed to read value
			return err
		} else if defaultSection == "" {
			// No value defined in database
			if _, ok := bc.Sections[bc.DefaultSection]; !ok {
				bc.DefaultSection = lo.Keys(bc.Sections)[0]
			}
			// Save to database
			if err = a.saveSettingValue(settingNameWithBlog(blog, defaultSectionSetting), bc.DefaultSection); err != nil {
				return err
			}
		} else {
			// Set value from database
			bc.DefaultSection = defaultSection
		}
		// Check if language is set
		if bc.Lang == "" {
			bc.Lang = "en"
		}
		// Blogroll
		if br := bc.Blogroll; br != nil && br.Enabled && br.Opml == "" {
			br.Enabled = false
		}
		// Load other settings from database
		configs := []*bool{
			&bc.hideOldContentWarning, &bc.hideShareButton, &bc.hideTranslateButton,
			&bc.addReplyTitle, &bc.addReplyContext, &bc.addLikeTitle, &bc.addLikeContext,
		}
		settings := []string{
			hideOldContentWarningSetting, hideShareButtonSetting, hideTranslateButtonSetting,
			addReplyTitleSetting, addReplyContextSetting, addLikeTitleSetting, addLikeContextSetting,
		}
		defaults := []bool{
			false, false, false,
			false, false, false, false,
		}
		for i := range configs {
			*configs[i], err = a.getBooleanSettingValue(settingNameWithBlog(blog, settings[i]), defaults[i])
			if err != nil {
				return err
			}
		}
	}
	// Log success
	a.cfg.initialized = true
	log.Println("Initialized configuration")
	return nil
}

func createDefaultConfig() *config {
	return &config{
		Server: &configServer{
			PublicAddress: "http://localhost:8080",
			LogFile:       "data/access.log",
		},
		Db: &configDb{
			File: "data/db.sqlite",
		},
		Cache: &configCache{
			Enable:     true,
			Expiration: 600,
		},
		User: &configUser{
			Nick:     "admin",
			Password: "secret",
		},
		Hooks: &configHooks{
			Shell: "/bin/bash",
		},
		Micropub: &configMicropub{
			CategoryParam:         "tags",
			ReplyParam:            "replylink",
			ReplyTitleParam:       "replytitle",
			ReplyContextParam:     "replycontext",
			LikeParam:             "likelink",
			LikeTitleParam:        "liketitle",
			LikeContextParam:      "likecontext",
			BookmarkParam:         "link",
			AudioParam:            "audio",
			PhotoParam:            "images",
			PhotoDescriptionParam: "imagealts",
			LocationParam:         "location",
		},
		ActivityPub: &configActivityPub{
			TagsTaxonomies: []string{"tags"},
		},
	}
}

func createDefaultBlog() *configBlog {
	return &configBlog{
		Path:        "/",
		Lang:        "en",
		Title:       "My Blog",
		Description: "Welcome to my blog.",
		Taxonomies: []*configTaxonomy{
			{
				Name:  "tags",
				Title: "Tags",
			},
		},
	}
}

func createDefaultSections() map[string]*configSection {
	return map[string]*configSection{
		"posts": {
			Title: "Posts",
		},
	}
}

func (a *goBlog) useSecureCookies() bool {
	return a.cfg.Server.SecurityHeaders || strings.HasPrefix(a.cfg.Server.PublicAddress, "https")
}

func (a *goBlog) getBlog(r *http.Request) (string, *configBlog) {
	if r == nil {
		return a.cfg.DefaultBlog, a.cfg.Blogs[a.cfg.DefaultBlog]
	}
	blog := a.cfg.DefaultBlog
	if ctxBlog := r.Context().Value(blogKey); ctxBlog != nil {
		if ctxBlogString, ok := ctxBlog.(string); ok {
			blog = ctxBlogString
		}
	}
	return blog, a.cfg.Blogs[blog]
}

func (a *goBlog) getBlogFromPost(p *post) *configBlog {
	return a.cfg.Blogs[defaultIfEmpty(p.Blog, a.cfg.DefaultBlog)]
}
