package main

import "log"

func main() {
	log.Println("Initializing configuration")
	err := initConfig()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Completed configuration")
	log.Println("Initializing database")
	err = initDatabase()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		log.Println("Close database")
		err := closeDb()
		if err != nil {
			log.Fatal(err)
		}
	}()
	log.Println("Loaded database")
	log.Println("Start server")
	err = startServer()
	if err != nil {
		log.Fatal(err)
	}
}
