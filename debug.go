package main

import "log"

func (a *goBlog) debug(msg ...any) {
	if a.cfg.Debug {
		log.Println(append([]any{"Debug:"}, msg...)...)
	}
}
