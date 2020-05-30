package pxeserver_test

import (
	"errors"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/ljfranklin/pxeserver"
	"github.com/stretchr/testify/assert"
)

func TestPixiecoreConfig(t *testing.T) {
	assert := assert.New(t)

	inputFile, err := os.Open(path.Join(fixturesDir(), "config", "simple.yaml"))
	assert.NoError(err)
	defer inputFile.Close()

	cfg, err := pxeserver.LoadConfig(inputFile)
	assert.NoError(err)

	actual := cfg.Pixiecore()

	mac := "52:54:00:12:34:56"
	host, ok := actual[pxeserver.MacAddress(mac)]
	assert.True(ok)
	assert.Contains(host.Kernel, mac)
	assert.Contains(host.Initrd[0], mac)
	assert.Regexp("preseed/url={{ file_url \"preseed\" }} preseed/url/checksum={{ file_md5 \"preseed\" }}", host.Cmdline)
}

func TestFilesConfig(t *testing.T) {
	assert := assert.New(t)

	inputFile, err := os.Open(path.Join(fixturesDir(), "config", "vars.yaml"))
	assert.NoError(err)
	defer inputFile.Close()

	cfg, err := pxeserver.LoadConfig(inputFile)
	assert.NoError(err)

	actual := cfg.Files()

	mac := "52:54:00:12:34:56"
	assert.Equal(len(actual), 3)

	kernel := actual[0]
	assert.Contains(kernel.ID, mac)
  assert.Contains(kernel.Path, "fixtures/x86_64/bzImage")
	assert.False(kernel.Template)
	assert.Nil(kernel.Vars)

	initrd := actual[1]
	assert.Contains(initrd.ID, mac)
  assert.Contains(initrd.Path, "fixtures/x86_64/netboot.cpio")
	assert.False(initrd.Template)
	assert.Nil(initrd.Vars)

	templateFile := actual[2]
	assert.Contains(templateFile.ID, mac)
  assert.Contains(templateFile.Path, "fixtures/vars.json")
	assert.True(templateFile.Template)
	assert.Equal(map[string]interface{}{
    "global_var": "global_value",
    "host_var": "host_value",
    "default_var": "default_value",
	}, templateFile.Vars)
}

func TestVarsForHost(t *testing.T) {
	assert := assert.New(t)

	inputFile, err := os.Open(path.Join(fixturesDir(), "config", "vars.yaml"))
	assert.NoError(err)
	defer inputFile.Close()

	cfg, err := pxeserver.LoadConfig(inputFile)
	assert.NoError(err)

	actual, err := cfg.VarsForHost("52:54:00:12:34:56")
	assert.NoError(err)

	assert.Equal(map[string]interface{}{
    "global_var": "global_value",
    "host_var": "host_value",
	}, actual)
}

func TestErrorOnBadReader(t *testing.T) {
	assert := assert.New(t)

	_, err := pxeserver.LoadConfig(badReader{})
	assert.NotNil(err)
	assert.Contains(err.Error(), "some-error")
}

func TestErrorOnNonYamlInput(t *testing.T) {
	assert := assert.New(t)

	badYaml := strings.NewReader("!!!!!")
	_, err := pxeserver.LoadConfig(badYaml)
	assert.NotNil(err)
	assert.Contains(err.Error(), "YAML")
}

func TestErrorIfVarsAreGivenButNotTemplate(t *testing.T) {
	assert := assert.New(t)

	inputFile, err := os.Open(path.Join(fixturesDir(), "config", "bad-vars-without-template.yaml"))
	assert.NoError(err)
	defer inputFile.Close()

	_, err = pxeserver.LoadConfig(inputFile)
	assert.NotNil(err)
	assert.Contains(err.Error(), "template: true")
}

func TestErrorOnMissingHost(t *testing.T) {
	assert := assert.New(t)

	inputFile, err := os.Open(path.Join(fixturesDir(), "config", "vars.yaml"))
	assert.NoError(err)
	defer inputFile.Close()

	cfg, err := pxeserver.LoadConfig(inputFile)
	assert.NoError(err)

	_, err = cfg.VarsForHost("some-missing-host")
	assert.NotNil(err)
	assert.Contains(err.Error(), "some-missing-host")
}

type badReader struct {}
func (b badReader) Read(p []byte) (int, error) {
	return 0, errors.New("some-error")
}

func fixturesDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return path.Join(path.Dir(filename), "fixtures")
}
