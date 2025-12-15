package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPostHasVideoPlaylist(t *testing.T) {
	t.Run("with playlist", func(t *testing.T) {
		p := &post{
			Parameters: map[string][]string{
				videoPlaylistParam: {"playlist-id"},
			},
		}
		assert.True(t, p.hasVideoPlaylist())
	})

	t.Run("without playlist", func(t *testing.T) {
		p := &post{
			Parameters: map[string][]string{},
		}
		assert.False(t, p.hasVideoPlaylist())
	})

	t.Run("empty playlist value", func(t *testing.T) {
		p := &post{
			Parameters: map[string][]string{
				videoPlaylistParam: {""},
			},
		}
		assert.False(t, p.hasVideoPlaylist())
	})
}
