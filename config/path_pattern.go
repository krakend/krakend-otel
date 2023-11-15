package config

import (
	"regexp"
	"strings"
)

var templatedURLPattern = regexp.MustCompile(`{{\.(.*?)}}`)

func NormalizeURLPattern(u string) string {
	s := templatedURLPattern.ReplaceAllStringFunc(u,
		func(x string) string {
			return ":" + strings.ToLower(x[3:len(x)-2])
		})
	return s
}
