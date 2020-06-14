module github.com/ljfranklin/pxeserver

go 1.14

replace go.universe.tf/netboot => github.com/ljfranklin/netboot v0.0.0-20200516152747-38439748f4c6

require (
	github.com/Masterminds/sprig/v3 v3.1.0
	github.com/ghodss/yaml v1.0.0
	github.com/imdario/mergo v0.3.9
	github.com/kevinburke/go-bindata v3.21.0+incompatible // indirect
	github.com/onsi/gomega v1.10.0
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.5.1
	go.universe.tf/netboot v0.0.0-20200205210610-68743c67a60c
	golang.org/x/crypto v0.0.0-20200414173820-0848c9571904
	gopkg.in/yaml.v2 v2.2.8
)
