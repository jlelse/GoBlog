package sample

import "fmt"

type plugin struct{}

func GetPlugin() fmt.Stringer {
	return plugin{}
}

func (plugin) String() string {
	return "ok"
}
