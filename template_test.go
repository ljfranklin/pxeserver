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

type MockSecrets struct {
	mock.Mock
}

func (m *MockSecrets) GetOrGenerate(mac string, id string) (interface{}, error) {
	args := m.Called(mac, id)
	return args.Get(0), args.Error(1)
}
func (m *MockSecrets) Get(mac string, id string) (interface{}, error) {
	args := m.Called(mac, id)
	return args.Get(0), args.Error(1)
}
func (m *MockSecrets) GetField(mac string, id string, field string) (interface{}, error) {
	args := m.Called(mac, id, field)
	return args.Get(0), args.Error(1)
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

func TestRenderFileWithSecrets(t *testing.T) {
	assert := assert.New(t)

	mockSecrets := new(MockSecrets)
	mockSecrets.On("GetOrGenerate", "some-mac", "some-id").Return("1234", nil)

	templateContents, err := ioutil.ReadFile(path.Join(fixturesDir(), "template", "secrets.txt"))
	assert.NoError(err)

	renderer := pxeserver.Renderer{
		Secrets: mockSecrets,
	}

	result, err := renderer.RenderFile(pxeserver.RenderFileArgs{
		Mac:      "some-mac",
		Template: string(templateContents),
		Vars: map[string]interface{}{
			"some_var": "{{ upper \"some_value\" }}",
		},
	})
	assert.NoError(err)

	assert.Equal("1234\n", result)
}

func TestRenderFileWithSharedSecrets(t *testing.T) {
	assert := assert.New(t)

	mockSecrets := new(MockSecrets)
	mockSecrets.On("GetOrGenerate", "", "some-id").Return("1234", nil)

	templateContents, err := ioutil.ReadFile(path.Join(fixturesDir(), "template", "shared-secrets.txt"))
	assert.NoError(err)

	renderer := pxeserver.Renderer{
		Secrets: mockSecrets,
	}

	result, err := renderer.RenderFile(pxeserver.RenderFileArgs{
		Mac:      "some-mac",
		Template: string(templateContents),
		Vars: map[string]interface{}{
			"some_var": "{{ upper \"some_value\" }}",
		},
	})
	assert.NoError(err)
	assert.Equal("1234\n", result)

	result, err = renderer.RenderFile(pxeserver.RenderFileArgs{
		Mac:      "some-other-mac",
		Template: string(templateContents),
		Vars: map[string]interface{}{
			"some_var": "{{ upper \"some_value\" }}",
		},
	})
	assert.NoError(err)
	assert.Equal("1234\n", result)
}

func TestRenderFileIgnoresFileDownloadHelpers(t *testing.T) {
	assert := assert.New(t)

	templateContents, err := ioutil.ReadFile(path.Join(fixturesDir(), "template", "simple.txt"))
	assert.NoError(err)

	renderer := pxeserver.Renderer{}

	result, err := renderer.RenderFile(pxeserver.RenderFileArgs{
		Template: string(templateContents),
		Vars: map[string]interface{}{
			"some_var":         "some_value",
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
		Vars:     map[string]interface{}{},
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

func TestRenderCmdlineWithSecrets(t *testing.T) {
	assert := assert.New(t)

	mockSecrets := new(MockSecrets)
	mockSecrets.On("GetOrGenerate", "some-mac", "some-id").Return("1234", nil)
	renderer := pxeserver.Renderer{
		Secrets: mockSecrets,
	}

	result, err := renderer.RenderCmdline(pxeserver.RenderCmdlineArgs{
		Mac:      "some-mac",
		Template: "some_boot_arg={{ secret \"some-id\" }}",
		Vars:     map[string]interface{}{},
	})
	assert.NoError(err)

	assert.Equal("some_boot_arg=1234", result)
}

func TestRenderCmdlineVarsWithSecrets(t *testing.T) {
	assert := assert.New(t)

	mockSecrets := new(MockSecrets)
	mockSecrets.On("GetOrGenerate", "some-mac", "some-id").Return("1234", nil)
	renderer := pxeserver.Renderer{
		Secrets: mockSecrets,
	}

	result, err := renderer.RenderCmdline(pxeserver.RenderCmdlineArgs{
		Mac:      "some-mac",
		Template: "some_boot_arg={{ .vars.some_var }}",
		Vars: map[string]interface{}{
			"some_var": "{{ secret \"some-id\" }}",
		},
	})
	assert.NoError(err)

	assert.Equal("some_boot_arg=1234", result)
}

func TestRenderCmdlineWithSharedSecrets(t *testing.T) {
	assert := assert.New(t)

	mockSecrets := new(MockSecrets)
	mockSecrets.On("GetOrGenerate", "", "some-id").Return("1234", nil)
	renderer := pxeserver.Renderer{
		Secrets: mockSecrets,
	}

	result, err := renderer.RenderCmdline(pxeserver.RenderCmdlineArgs{
		Mac:      "some-mac",
		Template: "some_boot_arg={{ shared_secret \"some-id\" }}",
		Vars:     map[string]interface{}{},
	})
	assert.NoError(err)
	assert.Equal("some_boot_arg=1234", result)

	result, err = renderer.RenderCmdline(pxeserver.RenderCmdlineArgs{
		Mac:      "some-other-mac",
		Template: "some_boot_arg={{ shared_secret \"some-id\" }}",
		Vars:     map[string]interface{}{},
	})
	assert.NoError(err)
	assert.Equal("some_boot_arg=1234", result)
}

func TestRenderCmdlineWithFiles(t *testing.T) {
	assert := assert.New(t)
	mockFiles := new(MockFiles)
	mockFiles.On("SHA256", "some_mac-some_file").Return("1234", nil)

	renderer := pxeserver.Renderer{}

	result, err := renderer.RenderCmdline(pxeserver.RenderCmdlineArgs{
		Template: "some_file={{ file_url \"some_file\" }} some_checksum={{ file_sha256 \"some_file\" }}",
		Vars:     map[string]interface{}{},
		Mac:      "some_mac",
		ExtraFuncs: template.FuncMap{
			"ID": func(string) string { return "some_url" },
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
		Vars:     map[string]interface{}{},
	})
	assert.NotNil(err)
	assert.Contains(err.Error(), "some_var")
}
