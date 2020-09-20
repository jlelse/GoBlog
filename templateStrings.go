package main

import (
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"path"
)

const stringsDir = "templates/strings"
const defaultVariant = "default"
const variantFileExt = ".yaml"

var templateStrings map[string]map[string]string

func initTemplateStrings() error {
	templateStrings = map[string]map[string]string{}
	for _, variant := range []string{defaultVariant} {
		variantStrings := map[string]string{}
		fileContent, err := ioutil.ReadFile(path.Join(stringsDir, variant+variantFileExt))
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(fileContent, variantStrings)
		if err != nil {
			return err
		}
		templateStrings[variant] = variantStrings
	}
	return nil
}

func getDefaultTemplateString(name string) string {
	return getTemplateStringVariant(name, defaultVariant)
}

func getTemplateStringVariant(name, variant string) string {
	return templateStrings[variant][name]
}
