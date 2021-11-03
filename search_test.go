package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_searchEncoding(t *testing.T) {

	testString := "test"

	assert.Equal(t, testString, searchDecode(searchEncode(testString)))

}
