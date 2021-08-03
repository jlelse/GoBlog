package main

import "log"

func (a *goBlog) debug(msg ...interface{}) {
	if a.cfg.Debug {
		log.Println(append([]interface{}{"Debug:"}, msg...)...)
	}
}
