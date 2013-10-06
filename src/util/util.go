package util

import (
	"os"
	"path"
)

func AbsPath(dirname string) string {
	if path.IsAbs(dirname) {
		return dirname
	}
	wd, _ := os.Getwd()
	return path.Join(wd, dirname)
}
