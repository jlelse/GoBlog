package main

import (
	"testing"

	ap "github.com/go-ap/activitypub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_apUsername(t *testing.T) {
	item, err := ap.UnmarshalJSON([]byte(`
	{
		"@context": [
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/v1"
		],
		"id": "https://example.org/users/user",
		"type": "Person",
		"preferredUsername": "user",
		"name": "Example user",
		"url": "https://example.org/@user"
	}
	`))
	require.NoError(t, err)

	actor, err := ap.ToActor(item)
	require.NoError(t, err)

	username := apUsername(actor)
	assert.Equal(t, "@user@example.org", username)
}
