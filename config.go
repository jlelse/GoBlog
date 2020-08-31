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
}

type configServer struct {
	Logging         bool   `mapstructure:"logging"`
	Port            int    `mapstructure:"port"`
	Domain          string `mapstructure:"domain"`
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
	// Number of posts per page
	Pagination int `mapstructure:"pagination"`
	// Sections
	Sections []string `mapstructure:"sections"`
	// Taxonomies
	Taxonomies []string `mapstructure:"taxonomies"`
}

type configUser struct {
	Nick     string `mapstructure:"nick"`
	Name     string `mapstructure:"name"`
	Password string `mapstructure:"password"`
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
	viper.SetDefault("server.publicHttps", false)
	viper.SetDefault("server.letsEncryptMail", "mail@example.com")
	viper.SetDefault("server.localHttps", false)
	viper.SetDefault("database.file", "data/db.sqlite")
	viper.SetDefault("cache.enable", true)
	viper.SetDefault("cache.expiration", 600)
	viper.SetDefault("blog.lang", "en")
	viper.SetDefault("blog.title", "My blog")
	viper.SetDefault("blog.pagination", 10)
	viper.SetDefault("blog.sections", []string{"posts"})
	viper.SetDefault("blog.taxonomies", []string{"tags"})
	viper.SetDefault("user.nick", "admin")
	viper.SetDefault("user.name", "Admin")
	viper.SetDefault("user.password", "secret")
	// Unmarshal config
	err = viper.Unmarshal(appConfig)
	if err != nil {
		return err
	}
	return nil
}
