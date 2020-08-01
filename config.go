package main

import (
	"github.com/spf13/viper"
)

type config struct {
	server *configServer
	db     *configDb
	cache  *configCache
	blog   *configBlog
	user   *configUser
}

type configServer struct {
	logging bool
	port    int
}

type configDb struct {
	file string
}

type configCache struct {
	enable     bool
	expiration int64
}

// exposed to templates via function "blog"
type configBlog struct {
	// Language of the blog, e.g. "en" or "de"
	Lang string
	// Title of the blog, e.g. "My blog"
	Title string
}

type configUser struct {
	nick     string
	name     string
	password string
}

var appConfig = &config{
	server: &configServer{},
	db:     &configDb{},
	cache:  &configCache{},
	blog:   &configBlog{},
	user:   &configUser{},
}

func initConfig() error {
	viper.SetConfigName("config")
	viper.AddConfigPath("./config/")
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}
	// Server
	serverLogging := "server.logging"
	viper.SetDefault(serverLogging, false)
	appConfig.server.logging = viper.GetBool(serverLogging)
	serverPort := "server.port"
	viper.SetDefault(serverPort, 8080)
	appConfig.server.port = viper.GetInt(serverPort)
	// Database
	databaseFile := "database.file"
	viper.SetDefault(databaseFile, "data/db.sqlite")
	appConfig.db.file = viper.GetString(databaseFile)
	// Caching
	cacheEnable := "cache.enable"
	viper.SetDefault(cacheEnable, true)
	appConfig.cache.enable = viper.GetBool(cacheEnable)
	cacheExpiration := "cache.expiration"
	viper.SetDefault(cacheExpiration, 600)
	appConfig.cache.expiration = viper.GetInt64(cacheExpiration)
	// Blog meta
	blogLang := "blog.lang"
	viper.SetDefault(blogLang, "en")
	appConfig.blog.Lang = viper.GetString(blogLang)
	blogTitle := "blog.title"
	viper.SetDefault(blogTitle, "My blog")
	appConfig.blog.Title = viper.GetString(blogTitle)
	// User
	userNick := "user.nick"
	viper.SetDefault(userNick, "admin")
	appConfig.user.nick = viper.GetString(userNick)
	userName := "user.name"
	viper.SetDefault(userName, "Admin")
	appConfig.user.name = viper.GetString(userName)
	userPassword := "user.password"
	viper.SetDefault(userPassword, "secret")
	appConfig.user.password = viper.GetString(userPassword)
	return nil
}
