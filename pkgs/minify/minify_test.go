package minify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_minify(t *testing.T) {
	var min Minifier
	assert.NotNil(t, min.Get())
}
