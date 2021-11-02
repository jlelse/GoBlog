package main

import "github.com/gorilla/websocket"

func (a *goBlog) webSocketUpgrader() *websocket.Upgrader {
	if a.wsUpgrader == nil {
		a.wsUpgrader = &websocket.Upgrader{
			EnableCompression: true,
		}
	}
	return a.wsUpgrader
}
