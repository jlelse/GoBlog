package main

import "embed"

const videoPlaylistParam = "videoplaylist"

//go:embed hlsjs/*
var hlsjsFiles embed.FS

func (p *post) hasVideoPlaylist() bool {
	return p.firstParameter(videoPlaylistParam) != ""
}
