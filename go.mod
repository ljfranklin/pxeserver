module github.com/ljfranklin/pxeserver

go 1.16

require (
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/ghodss/yaml v1.0.0
	github.com/imdario/mergo v0.3.11
	github.com/onsi/gomega v1.10.0
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.5.1
	go.universe.tf/netboot v0.0.0-00010101000000-000000000000
	golang.org/x/crypto v0.1.0
	gopkg.in/yaml.v2 v2.3.0
)

replace go.universe.tf/netboot => github.com/ljfranklin/netboot v0.0.0-20210227200705-32fe5569bce0
