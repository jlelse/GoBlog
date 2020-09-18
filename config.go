package main

import (
	"github.com/spf13/viper"
)

type config struct {
	Server *configServer `mapstructure:"server"`
	Db     *configDb     `mapstructure:"database"`
	Cache  *configCache  `mapstructure:"cache"`
	Blog   *configBlog   `mapstructure:"blog"`
	User   *configUser   `mapstructure:"user"`
	Hugo   *configHugo   `mapstructure:"hugo"`
}

type configServer struct {
	Logging         bool   `mapstructure:"logging"`
	Port            int    `mapstructure:"port"`
	Domain          string `mapstructure:"domain"`
	PublicAddress   string `mapstructure:"publicAddress"`
	PublicHttps     bool   `mapstructure:"publicHttps"`
	LetsEncryptMail string `mapstructure:"letsEncryptMail"`
	LocalHttps      bool   `mapstructure:"localHttps"`
}

type configDb struct {
	File string `mapstructure:"file"`
}

type configCache struct {
	Enable     bool  `mapstructure:"enable"`
	Expiration int64 `mapstructure:"expiration"`
}

// exposed to templates via function "blog"
type configBlog struct {
	// Language of the blog, e.g. "en" or "de"
	Lang string `mapstructure:"lang"`
	// Title of the blog, e.g. "My blog"
	Title string `mapstructure:"title"`
	// Description of the blog
	Description string `mapstructure:"description"`
	// Number of posts per page
	Pagination int `mapstructure:"pagination"`
	// Sections
	Sections []*section `mapstructure:"sections"`
	// Taxonomies
	Taxonomies []*taxonomy `mapstructure:"taxonomies"`
}

type section struct {
	Name        string `mapstructure:"name"`
	Title       string `mapstructure:"title"`
	Description string `mapstructure:"description"`
}

type taxonomy struct {
	Name        string `mapstructure:"name"`
	Title       string `mapstructure:"title"`
	Description string `mapstructure:"description"`
}

type configUser struct {
	Nick     string `mapstructure:"nick"`
	Name     string `mapstructure:"name"`
	Password string `mapstructure:"password"`
}

type configHugo struct {
	Frontmatter []*frontmatter `mapstructure:"frontmatter"`
}

type frontmatter struct {
	Meta      string `mapstructure:"meta"`
	Parameter string `mapstructure:"parameter"`
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
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.domain", "example.com")
	viper.SetDefault("server.publicAddress", "http://localhost:8080")
	viper.SetDefault("server.publicHttps", false)
	viper.SetDefault("server.letsEncryptMail", "mail@example.com")
	viper.SetDefault("server.localHttps", false)
	viper.SetDefault("database.file", "data/db.sqlite")
	viper.SetDefault("cache.enable", true)
	viper.SetDefault("cache.expiration", 600)
	viper.SetDefault("blog.lang", "en")
	viper.SetDefault("blog.title", "My blog")
	viper.SetDefault("blog.description", "This is my blog")
	viper.SetDefault("blog.pagination", 10)
	viper.SetDefault("blog.sections", []*section{{Name: "posts", Title: "Posts", Description: "**Posts** on this blog"}})
	viper.SetDefault("blog.taxonomies", []*taxonomy{{Name: "tags", Title: "Tags", Description: "**Tags** on this blog"}})
	viper.SetDefault("user.nick", "admin")
	viper.SetDefault("user.name", "Admin")
	viper.SetDefault("user.password", "secret")
	viper.SetDefault("hugo.frontmatter", []*frontmatter{{Meta: "title", Parameter: "title"}, {Meta: "tags", Parameter: "tags"}})
	// Unmarshal config
	err = viper.Unmarshal(appConfig)
	if err != nil {
		return err
	}
	return nil
}
