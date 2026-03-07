// Package sample provides a sample GoBlog plugin.
package sample

import "fmt"

type plugin struct{}

// GetPlugin returns the sample plugin instance.
func GetPlugin() fmt.Stringer {
	return plugin{}
}

func (plugin) String() string {
	return "ok"
}
