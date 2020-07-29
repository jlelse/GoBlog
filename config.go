package main

import (
	"github.com/spf13/viper"
	"log"
)

type config struct {
	server *configServer
	db     *configDb
	cache  *configCache
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

var appConfig = &config{
	server: &configServer{},
	db:     &configDb{},
	cache:  &configCache{},
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
	return nil
}

func logConfig(key string, value interface{}) {
	log.Println(key+":", value)
}
