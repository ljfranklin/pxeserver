package pxeserver

import (
	"embed"
)

//go:embed bindeps
var bindeps embed.FS

func readAsset(filename string) ([]byte, error) {
	return bindeps.ReadFile(filename)
}
