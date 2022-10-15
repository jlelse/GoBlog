package main

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getFullAddress(t *testing.T) {
	cfg1 := &configServer{
		PublicAddress: "https://example.com",
	}

	cfg2 := &configServer{
		PublicAddress: "https://example.com/",
	}

	app := &goBlog{
		cfg: &config{
			Server: cfg1,
		},
	}

	if got := cfg1.getFullAddress("/test"); !reflect.DeepEqual(got, "https://example.com/test") {
		t.Errorf("Wrong full path, got: %v", got)
	}

	if got := cfg2.getFullAddress("/test"); !reflect.DeepEqual(got, "https://example.com/test") {
		t.Errorf("Wrong full path, got: %v", got)
	}

	if got := app.getFullAddress("/test"); !reflect.DeepEqual(got, "https://example.com/test") {
		t.Errorf("Wrong full path, got: %v", got)
	}

	if got := cfg1.getFullAddress("/"); !reflect.DeepEqual(got, "https://example.com") {
		t.Errorf("Wrong full path, got: %v", got)
	}

	if got := cfg1.getFullAddress(""); !reflect.DeepEqual(got, "https://example.com") {
		t.Errorf("Wrong full path, got: %v", got)
	}

	assert.Equal(t, "https://example.net", cfg1.getFullAddress("https://example.net"))
	assert.Equal(t, "https://example.net", cfg2.getFullAddress("https://example.net"))

	assert.Equal(t, "https://example.com/", app.getInstanceRootURL())
}

func Test_getRelativeBlogPath(t *testing.T) {
	blog1 := &configBlog{
		Path: "",
	}

	blog2 := &configBlog{
		Path: "",
	}

	blog3 := &configBlog{
		Path: "/de",
	}

	if got := blog1.getRelativePath(""); !reflect.DeepEqual(got, "/") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}

	if got := blog2.getRelativePath(""); !reflect.DeepEqual(got, "/") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}

	if got := blog3.getRelativePath(""); !reflect.DeepEqual(got, "/de") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}

	if got := blog1.getRelativePath("test"); !reflect.DeepEqual(got, "/test") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}

	if got := blog2.getRelativePath("test"); !reflect.DeepEqual(got, "/test") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}

	if got := blog3.getRelativePath("test"); !reflect.DeepEqual(got, "/de/test") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}

	if got := blog1.getRelativePath("/test"); !reflect.DeepEqual(got, "/test") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}

	if got := blog2.getRelativePath("/test"); !reflect.DeepEqual(got, "/test") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}

	if got := blog3.getRelativePath("/test"); !reflect.DeepEqual(got, "/de/test") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}

	app := &goBlog{
		cfg: &config{
			Blogs: map[string]*configBlog{
				"de": blog3,
			},
		},
	}

	if got := app.getRelativePath("de", "/test"); !reflect.DeepEqual(got, "/de/test") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}

	if got := app.getRelativePath("", "/test"); !reflect.DeepEqual(got, "") {
		t.Errorf("Wrong relative blog path, got: %v", got)
	}
}
