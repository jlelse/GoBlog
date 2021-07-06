package main

import (
	"errors"
	"log"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

type config struct {
	Server        *configServer          `mapstructure:"server"`
	Db            *configDb              `mapstructure:"database"`
	Cache         *configCache           `mapstructure:"cache"`
	DefaultBlog   string                 `mapstructure:"defaultblog"`
	Blogs         map[string]*configBlog `mapstructure:"blogs"`
	User          *configUser            `mapstructure:"user"`
	Hooks         *configHooks           `mapstructure:"hooks"`
	Micropub      *configMicropub        `mapstructure:"micropub"`
	PathRedirects []*configRegexRedirect `mapstructure:"pathRedirects"`
	ActivityPub   *configActivityPub     `mapstructure:"activityPub"`
	Webmention    *configWebmention      `mapstructure:"webmention"`
	Notifications *configNotifications   `mapstructure:"notifications"`
	PrivateMode   *configPrivateMode     `mapstructure:"privateMode"`
}

type configServer struct {
	Logging             bool     `mapstructure:"logging"`
	LogFile             string   `mapstructure:"logFile"`
	Port                int      `mapstructure:"port"`
	PublicAddress       string   `mapstructure:"publicAddress"`
	ShortPublicAddress  string   `mapstructure:"shortPublicAddress"`
	PublicHTTPS         bool     `mapstructure:"publicHttps"`
	Tor                 bool     `mapstructure:"tor"`
	SecurityHeaders     bool     `mapstructure:"securityHeaders"`
	CSPDomains          []string `mapstructure:"cspDomains"`
	JWTSecret           string   `mapstructure:"jwtSecret"`
	publicHostname      string
	shortPublicHostname string
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
	Path           string              `mapstructure:"path"`
	Lang           string              `mapstructure:"lang"`
	Title          string              `mapstructure:"title"`
	Description    string              `mapstructure:"description"`
	Pagination     int                 `mapstructure:"pagination"`
	DefaultSection string              `mapstructure:"defaultsection"`
	Sections       map[string]*section `mapstructure:"sections"`
	Taxonomies     []*taxonomy         `mapstructure:"taxonomies"`
	Menus          map[string]*menu    `mapstructure:"menus"`
	Photos         *photos             `mapstructure:"photos"`
	Search         *search             `mapstructure:"search"`
	BlogStats      *blogStats          `mapstructure:"blogStats"`
	Blogroll       *configBlogroll     `mapstructure:"blogroll"`
	CustomPages    []*customPage       `mapstructure:"custompages"`
	Telegram       *configTelegram     `mapstructure:"telegram"`
	PostAsHome     bool                `mapstructure:"postAsHome"`
	RandomPost     *randomPost         `mapstructure:"randomPost"`
	Comments       *comments           `mapstructure:"comments"`
	Map            *configMap          `mapstructure:"map"`
}

type section struct {
	Name         string `mapstructure:"name"`
	Title        string `mapstructure:"title"`
	Description  string `mapstructure:"description"`
	PathTemplate string `mapstructure:"pathtemplate"`
}

type taxonomy struct {
	Name        string `mapstructure:"name"`
	Title       string `mapstructure:"title"`
	Description string `mapstructure:"description"`
}

type menu struct {
	Items []*menuItem `mapstructure:"items"`
}

type menuItem struct {
	Title string `mapstructure:"title"`
	Link  string `mapstructure:"link"`
}

type photos struct {
	Enabled     bool   `mapstructure:"enabled"`
	Parameter   string `mapstructure:"parameter"`
	Path        string `mapstructure:"path"`
	Title       string `mapstructure:"title"`
	Description string `mapstructure:"description"`
}

type search struct {
	Enabled     bool   `mapstructure:"enabled"`
	Path        string `mapstructure:"path"`
	Title       string `mapstructure:"title"`
	Description string `mapstructure:"description"`
	Placeholder string `mapstructure:"placeholder"`
}

type blogStats struct {
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

type customPage struct {
	Path            string       `mapstructure:"path"`
	Template        string       `mapstructure:"template"`
	Cache           bool         `mapstructure:"cache"`
	CacheExpiration int          `mapstructure:"cacheExpiration"`
	Data            *interface{} `mapstructure:"data"`
}

type randomPost struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type comments struct {
	Enabled bool `mapstructure:"enabled"`
}

type configMap struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

type configUser struct {
	Nick         string         `mapstructure:"nick"`
	Name         string         `mapstructure:"name"`
	Password     string         `mapstructure:"password"`
	TOTP         string         `mapstructure:"totp"`
	AppPasswords []*appPassword `mapstructure:"appPasswords"`
	Picture      string         `mapstructure:"picture"`
	Email        string         `mapstructure:"email"`
	Link         string         `mapstructure:"link"`
	Identities   []string       `mapstructure:"identities"`
}

type appPassword struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type configHooks struct {
	Shell    string   `mapstructure:"shell"`
	Hourly   []string `mapstructure:"hourly"`
	PreStart []string `mapstructure:"prestart"`
	// Can use template
	PostPost   []string `mapstructure:"postpost"`
	PostUpdate []string `mapstructure:"postupdate"`
	PostDelete []string `mapstructure:"postdelete"`
}

type configMicropub struct {
	CategoryParam         string               `mapstructure:"categoryParam"`
	ReplyParam            string               `mapstructure:"replyParam"`
	LikeParam             string               `mapstructure:"likeParam"`
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
	// Shortpixel
	ShortPixelKey string `mapstructure:"shortPixelKey"`
	// Cloudflare
	CloudflareCompressionEnabled bool `mapstructure:"cloudflareCompressionEnabled"`
}

type configRegexRedirect struct {
	From string `mapstructure:"from"`
	To   string `mapstructure:"to"`
	Type int    `mapstructure:"type"`
}

type configActivityPub struct {
	Enabled        bool     `mapstructure:"enabled"`
	KeyPath        string   `mapstructure:"keyPath"`
	TagsTaxonomies []string `mapstructure:"tagsTaxonomies"`
}

type configNotifications struct {
	Telegram *configTelegram `mapstructure:"telegram"`
}

type configTelegram struct {
	Enabled         bool   `mapstructure:"enabled"`
	ChatID          string `mapstructure:"chatId"`
	BotToken        string `mapstructure:"botToken"`
	InstantViewHash string `mapstructure:"instantViewHash"`
}

type configPrivateMode struct {
	Enabled bool `mapstructure:"enabled"`
}

type configWebmention struct {
	DisableSending   bool `mapstructure:"disableSending"`
	DisableReceiving bool `mapstructure:"disableReceiving"`
}

func (a *goBlog) initConfig() error {
	log.Println("Initialize configuration...")
	viper.SetConfigName("config")
	viper.AddConfigPath("./config/")
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}
	// Defaults
	viper.SetDefault("server.logging", false)
	viper.SetDefault("server.logFile", "data/access.log")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.publicAddress", "http://localhost:8080")
	viper.SetDefault("server.publicHttps", false)
	viper.SetDefault("database.file", "data/db.sqlite")
	viper.SetDefault("cache.enable", true)
	viper.SetDefault("cache.expiration", 600)
	viper.SetDefault("user.nick", "admin")
	viper.SetDefault("user.password", "secret")
	viper.SetDefault("hooks.shell", "/bin/bash")
	viper.SetDefault("micropub.categoryParam", "tags")
	viper.SetDefault("micropub.replyParam", "replylink")
	viper.SetDefault("micropub.likeParam", "likelink")
	viper.SetDefault("micropub.bookmarkParam", "link")
	viper.SetDefault("micropub.audioParam", "audio")
	viper.SetDefault("micropub.photoParam", "images")
	viper.SetDefault("micropub.photoDescriptionParam", "imagealts")
	viper.SetDefault("micropub.locationParam", "location")
	viper.SetDefault("activityPub.keyPath", "data/private.pem")
	viper.SetDefault("activityPub.tagsTaxonomies", []string{"tags"})
	viper.SetDefault("webmention.disableSending", false)
	viper.SetDefault("webmention.disableReceiving", false)
	// Unmarshal config
	a.cfg = &config{}
	err = viper.Unmarshal(a.cfg)
	if err != nil {
		return err
	}
	// Check config
	publicURL, err := url.Parse(a.cfg.Server.PublicAddress)
	if err != nil {
		return err
	}
	a.cfg.Server.publicHostname = publicURL.Hostname()
	if a.cfg.Server.ShortPublicAddress != "" {
		shortPublicURL, err := url.Parse(a.cfg.Server.ShortPublicAddress)
		if err != nil {
			return err
		}
		a.cfg.Server.shortPublicHostname = shortPublicURL.Hostname()
	}
	if a.cfg.Server.JWTSecret == "" {
		return errors.New("no JWT secret configured")
	}
	if len(a.cfg.Blogs) == 0 {
		return errors.New("no blog configured")
	}
	if len(a.cfg.DefaultBlog) == 0 || a.cfg.Blogs[a.cfg.DefaultBlog] == nil {
		return errors.New("no default blog or default blog not present")
	}
	if a.cfg.Micropub.MediaStorage != nil {
		if a.cfg.Micropub.MediaStorage.MediaURL == "" ||
			a.cfg.Micropub.MediaStorage.BunnyStorageKey == "" ||
			a.cfg.Micropub.MediaStorage.BunnyStorageName == "" {
			a.cfg.Micropub.MediaStorage.BunnyStorageKey = ""
			a.cfg.Micropub.MediaStorage.BunnyStorageName = ""
		}
		a.cfg.Micropub.MediaStorage.MediaURL = strings.TrimSuffix(a.cfg.Micropub.MediaStorage.MediaURL, "/")
	}
	if pm := a.cfg.PrivateMode; pm != nil && pm.Enabled {
		a.cfg.ActivityPub = &configActivityPub{Enabled: false}
	}
	if wm := a.cfg.Webmention; wm != nil && wm.DisableReceiving {
		// Disable comments for all blogs
		for _, b := range a.cfg.Blogs {
			b.Comments = &comments{Enabled: false}
		}
	}
	// Check config for each blog
	for _, blog := range a.cfg.Blogs {
		if br := blog.Blogroll; br != nil && br.Enabled && br.Opml == "" {
			br.Enabled = false
		}
	}
	log.Println("Initialized configuration")
	return nil
}

func (a *goBlog) httpsConfigured() bool {
	return a.cfg.Server.PublicHTTPS || a.cfg.Server.SecurityHeaders || strings.HasPrefix(a.cfg.Server.PublicAddress, "https")
}
