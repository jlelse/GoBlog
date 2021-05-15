package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const stringsDir = "templates/strings"
const defaultStrings = "default"
const variantFileExt = ".yaml"

var templateStrings map[string]map[string]string

func initTemplateStrings() error {
	templateStrings = map[string]map[string]string{}
	variants := []string{defaultStrings}
	for _, blog := range appConfig.Blogs {
		variants = append(variants, blog.Lang)
	}
	for _, variant := range variants {
		variantStrings := map[string]string{}
		f, err := os.Open(filepath.Join(stringsDir, variant+variantFileExt))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		err = yaml.NewDecoder(f).Decode(variantStrings)
		if err != nil {
			return err
		}
		templateStrings[variant] = variantStrings
	}
	return nil
}

func getTemplateStringVariant(input ...string) (result string) {
	var lang, name string
	if l := len(input); l == 1 {
		lang = appConfig.Blogs[appConfig.DefaultBlog].Lang
		name = input[0]
	} else if l == 2 {
		lang = input[0]
		name = input[1]
	} else {
		// Wrong number of input strings
		return ""
	}
	m, ok := templateStrings[lang]
	if !ok {
		m = templateStrings[defaultStrings]
	}
	result, ok = m[name]
	if !ok {
		result = templateStrings[defaultStrings][name]
	}
	return
}
