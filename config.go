package main

import (
	"errors"
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
	Hugo          *configHugo            `mapstructure:"hugo"`
	Micropub      *configMicropub        `mapstructure:"micropub"`
	PathRedirects []*configRegexRedirect `mapstructure:"pathRedirects"`
}

type configServer struct {
	Logging         bool   `mapstructure:"logging"`
	Debug           bool   `mapstructure:"Debug"`
	Port            int    `mapstructure:"port"`
	Domain          string `mapstructure:"domain"`
	PublicAddress   string `mapstructure:"publicAddress"`
	PublicHTTPS     bool   `mapstructure:"publicHttps"`
	LetsEncryptMail string `mapstructure:"letsEncryptMail"`
	LocalHTTPS      bool   `mapstructure:"localHttps"`
}

type configDb struct {
	File string `mapstructure:"file"`
}

type configCache struct {
	Enable     bool  `mapstructure:"enable"`
	Expiration int64 `mapstructure:"expiration"`
}

type configBlog struct {
	Path            string              `mapstructure:"path"`
	Lang            string              `mapstructure:"lang"`
	Title           string              `mapstructure:"title"`
	Description     string              `mapstructure:"description"`
	Pagination      int                 `mapstructure:"pagination"`
	Sections        map[string]*section `mapstructure:"sections"`
	Taxonomies      []*taxonomy         `mapstructure:"taxonomies"`
	Menus           map[string]*menu    `mapstructure:"menus"`
	Photos          *photos             `mapstructure:"photos"`
	ActivityStreams *activityStreams    `mapstructure:"activitystreams"`
	DefaultSection  string              `mapstructure:"defaultsection"`
	CustomPages     []*customPage       `mapstructure:"custompages"`
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

type activityStreams struct {
	Enabled         bool   `mapstructure:"enabled"`
	ReplyParameter  string `mapstructure:"replyParameter"`
	ImagesParameter string `mapstructure:"imagesParameter"`
}

type customPage struct {
	Path     string       `mapstructure:"path"`
	Template string       `mapstructure:"template"`
	Cache    bool         `mapstructure:"cache"`
	Data     *interface{} `mapstructure:"data"`
}

type configUser struct {
	Nick     string `mapstructure:"nick"`
	Name     string `mapstructure:"name"`
	Password string `mapstructure:"password"`
}

type configHooks struct {
	Shell    string   `mapstructure:"shell"`
	PreStart []string `mapstructure:"prestart"`
}

type configHugo struct {
	Frontmatter []*frontmatter `mapstructure:"frontmatter"`
}

type frontmatter struct {
	Meta      string `mapstructure:"meta"`
	Parameter string `mapstructure:"parameter"`
}

type configMicropub struct {
	CategoryParam         string               `mapstructure:"categoryParam"`
	ReplyParam            string               `mapstructure:"replyParam"`
	LikeParam             string               `mapstructure:"likeParam"`
	BookmarkParam         string               `mapstructure:"bookmarkParam"`
	AudioParam            string               `mapstructure:"audioParam"`
	PhotoParam            string               `mapstructure:"photoParam"`
	PhotoDescriptionParam string               `mapstructure:"photoDescriptionParam"`
	MediaStorage          *configMicropubMedia `mapstructure:"mediaStorage"`
}

type configMicropubMedia struct {
	MediaURL         string `mapstructure:"mediaUrl"`
	BunnyStorageKey  string `mapstructure:"bunnyStorageKey"`
	BunnyStorageName string `mapstructure:"bunnyStorageName"`
	TinifyKey        string `mapstructure:"tinifyKey"`
}

type configRegexRedirect struct {
	From string `mapstructure:"from"`
	To   string `mapstructure:"to"`
}

var appConfig = &config{}

func initConfig() error {
	viper.SetConfigName("config")
	viper.AddConfigPath("./config/")
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}
	// Defaults
	viper.SetDefault("server.logging", false)
	viper.SetDefault("server.debug", false)
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.publicAddress", "http://localhost:8080")
	viper.SetDefault("server.publicHttps", false)
	viper.SetDefault("server.localHttps", false)
	viper.SetDefault("database.file", "data/db.sqlite")
	viper.SetDefault("cache.enable", true)
	viper.SetDefault("cache.expiration", 600)
	viper.SetDefault("user.nick", "admin")
	viper.SetDefault("user.name", "Admin")
	viper.SetDefault("user.password", "secret")
	viper.SetDefault("hooks.shell", "/bin/bash")
	viper.SetDefault("hugo.frontmatter", []*frontmatter{{Meta: "title", Parameter: "title"}, {Meta: "tags", Parameter: "tags"}})
	viper.SetDefault("micropub.categoryParam", "tags")
	viper.SetDefault("micropub.replyParam", "replylink")
	viper.SetDefault("micropub.likeParam", "likelink")
	viper.SetDefault("micropub.bookmarkParam", "link")
	viper.SetDefault("micropub.audioParam", "audio")
	viper.SetDefault("micropub.photoParam", "images")
	viper.SetDefault("micropub.photoDescriptionParam", "imagealts")
	// Unmarshal config
	err = viper.Unmarshal(appConfig)
	if err != nil {
		return err
	}
	// Check config
	if appConfig.Server.Domain == "" {
		return errors.New("no domain configured")
	}
	if len(appConfig.Blogs) == 0 {
		return errors.New("no blog configured")
	}
	if len(appConfig.DefaultBlog) == 0 || appConfig.Blogs[appConfig.DefaultBlog] == nil {
		return errors.New("no default blog or default blog not present")
	}
	if appConfig.Micropub.MediaStorage != nil {
		if appConfig.Micropub.MediaStorage.MediaURL == "" ||
			appConfig.Micropub.MediaStorage.BunnyStorageKey == "" ||
			appConfig.Micropub.MediaStorage.BunnyStorageName == "" {
			appConfig.Micropub.MediaStorage = nil
		} else {
			appConfig.Micropub.MediaStorage.MediaURL = strings.TrimSuffix(appConfig.Micropub.MediaStorage.MediaURL, "/")
		}
	}
	return nil
}
