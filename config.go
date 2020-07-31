package main

import (
	"github.com/spf13/viper"
	"log"
)

type config struct {
	server *configServer
	db     *configDb
	cache  *configCache
	blog   *configBlog
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
	Lang  string
	// Title of the blog, e.g. "My blog"
	Title string
}

var appConfig = &config{
	server: &configServer{},
	db:     &configDb{},
	cache:  &configCache{},
	blog:   &configBlog{},
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
	logConfig(serverLogging, appConfig.server.logging)
	serverPort := "server.port"
	viper.SetDefault(serverPort, 8080)
	appConfig.server.port = viper.GetInt(serverPort)
	logConfig(serverPort, appConfig.server.port)
	// Database
	databaseFile := "database.file"
	viper.SetDefault(databaseFile, "data/db.sqlite")
	appConfig.db.file = viper.GetString(databaseFile)
	logConfig(databaseFile, appConfig.db.file)
	// Caching
	cacheEnable := "cache.enable"
	viper.SetDefault(cacheEnable, true)
	appConfig.cache.enable = viper.GetBool(cacheEnable)
	logConfig(cacheEnable, appConfig.cache.enable)
	cacheExpiration := "cache.expiration"
	viper.SetDefault(cacheExpiration, 600)
	appConfig.cache.expiration = viper.GetInt64(cacheExpiration)
	logConfig(cacheExpiration, appConfig.cache.expiration)
	// Blog meta
	blogLang := "blog.lang"
	viper.SetDefault(blogLang, "en")
	appConfig.blog.Lang = viper.GetString(blogLang)
	logConfig(blogLang, appConfig.blog.Lang)
	blogTitle := "blog.title"
	viper.SetDefault(blogTitle, "My blog")
	appConfig.blog.Title = viper.GetString(blogTitle)
	logConfig(blogTitle, appConfig.blog.Title)
	return nil
}

func logConfig(key string, value interface{}) {
	log.Println(key+":", value)
}
