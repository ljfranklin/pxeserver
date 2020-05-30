package pxeserver_test

import (
	"io/ioutil"
	"path"
	"testing"
	"text/template"

	"github.com/ljfranklin/pxeserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockFiles struct {
	mock.Mock
}
func (m *MockFiles) SHA256(id string) (string, error) {
  args := m.Called(id)
  return args.String(0), args.Error(1)
}
func (m *MockFiles) MD5(id string) (string, error) {
  args := m.Called(id)
  return args.String(0), args.Error(1)
}

func TestRenderFileSimple(t *testing.T) {
	assert := assert.New(t)

	templateContents, err := ioutil.ReadFile(path.Join(fixturesDir(), "template", "simple.txt"))
	assert.NoError(err)

	renderer := pxeserver.Renderer{}

	result, err := renderer.RenderFile(pxeserver.RenderFileArgs{
		Template: string(templateContents),
		Vars: map[string]interface{}{
			"some_var": "some_value",
		},
	})
	assert.NoError(err)

	assert.Equal("some-text\nsome_value\n4\n", result)
}

func TestRenderFileWithTemplatedVars(t *testing.T) {
	assert := assert.New(t)

	templateContents, err := ioutil.ReadFile(path.Join(fixturesDir(), "template", "simple.txt"))
	assert.NoError(err)

	renderer := pxeserver.Renderer{}

	result, err := renderer.RenderFile(pxeserver.RenderFileArgs{
		Template: string(templateContents),
		Vars: map[string]interface{}{
			"some_var": "{{ upper \"some_value\" }}",
		},
	})
	assert.NoError(err)

	assert.Equal("some-text\nSOME_VALUE\n4\n", result)
}

func TestRenderFileIgnoresFileDownloadHelpers(t *testing.T) {
	assert := assert.New(t)

	templateContents, err := ioutil.ReadFile(path.Join(fixturesDir(), "template", "simple.txt"))
	assert.NoError(err)

	renderer := pxeserver.Renderer{}

	result, err := renderer.RenderFile(pxeserver.RenderFileArgs{
		Template: string(templateContents),
		Vars: map[string]interface{}{
			"some_var": "some_value",
			"some_ignored_var": "{{ file_url \"some-file\" }}",
		},
	})
	assert.NoError(err)

	assert.Equal("some-text\nsome_value\n4\n", result)
}

func TestRenderFileErrorOnMissingVar(t *testing.T) {
	assert := assert.New(t)

	templateContents, err := ioutil.ReadFile(path.Join(fixturesDir(), "template", "simple.txt"))
	assert.NoError(err)

	renderer := pxeserver.Renderer{}

	_, err = renderer.RenderFile(pxeserver.RenderFileArgs{
		Template: string(templateContents),
		Vars: map[string]interface{}{},
	})
	assert.NotNil(err)
	assert.Contains(err.Error(), "some_var")
}

func TestRenderCmdlineSimple(t *testing.T) {
	assert := assert.New(t)

	renderer := pxeserver.Renderer{}

	result, err := renderer.RenderCmdline(pxeserver.RenderCmdlineArgs{
		Template: "some_boot_arg={{ .vars.some_var }}",
		Vars: map[string]interface{}{
			"some_var": "some_value",
		},
	})
	assert.NoError(err)

	assert.Equal("some_boot_arg=some_value", result)
}

func TestRenderCmdlineWithTemplatedVars(t *testing.T) {
	assert := assert.New(t)

	renderer := pxeserver.Renderer{}

	result, err := renderer.RenderCmdline(pxeserver.RenderCmdlineArgs{
		Template: "some_boot_arg={{ .vars.some_var }}",
		Vars: map[string]interface{}{
			"some_var": "{{ upper \"some_value\" }}",
		},
	})
	assert.NoError(err)

	assert.Equal("some_boot_arg=SOME_VALUE", result)
}

func TestRenderCmdlineWithFiles(t *testing.T) {
	assert := assert.New(t)
	mockFiles := new(MockFiles)
	mockFiles.On("SHA256", "some_mac-some_file").Return("1234", nil)

	renderer := pxeserver.Renderer{}

	result, err := renderer.RenderCmdline(pxeserver.RenderCmdlineArgs{
		Template: "some_file={{ file_url \"some_file\" }} some_checksum={{ file_sha256 \"some_file\" }}",
		Vars: map[string]interface{}{},
		Mac: "some_mac",
		ExtraFuncs: template.FuncMap{
			"ID": func(string) (string) { return "some_url" },
		},
		Files: mockFiles,
	})
	assert.NoError(err)

	assert.Equal("some_file=some_url some_checksum=1234", result)
}

func TestRenderCmdlineErrorOnMissingVar(t *testing.T) {
	assert := assert.New(t)

	renderer := pxeserver.Renderer{}

	_, err := renderer.RenderCmdline(pxeserver.RenderCmdlineArgs{
		Template: "some_boot_arg={{ .vars.some_var }}",
		Vars: map[string]interface{}{},
	})
	assert.NotNil(err)
	assert.Contains(err.Error(), "some_var")
}
