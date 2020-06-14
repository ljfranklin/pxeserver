package pxeserver_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/ljfranklin/pxeserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockRenderer struct {
	mock.Mock
}

func (m *MockRenderer) RenderFile(f pxeserver.RenderFileArgs) (string, error) {
	args := m.Called(f)
	return args.String(0), args.Error(1)
}
func (m *MockRenderer) RenderPath(s string) (string, error) {
	args := m.Called(s)
	return args.String(0), args.Error(1)
}

func TestSimpleRead(t *testing.T) {
	assert := assert.New(t)

	fixturePath := path.Join(fixturesDir(), "files", "simple.txt")

	mockRenderer := new(MockRenderer)
	mockRenderer.On("RenderPath", fixturePath).Return(fixturePath, nil)

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:   "some-id",
			Path: fixturePath,
		},
	}, mockRenderer)
	assert.NoError(err)

	fileReader, fileSize, err := f.Read("some-id")
	assert.NoError(err)
	defer fileReader.Close()

	fileContents, err := ioutil.ReadAll(fileReader)
	assert.NoError(err)

	assert.Equal([]byte("some-text\n"), fileContents)
	assert.Equal(int64(len("some-text\n")), fileSize)

	mockRenderer.AssertNumberOfCalls(t, "RenderFile", 0)
}

func TestVarsRead(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)
	fixturePath := path.Join(fixturesDir(), "files", "vars.txt")

	mockRenderer.On("RenderPath", fixturePath).Return(fixturePath, nil)
	expectedFileArgs := pxeserver.RenderFileArgs{
		Template: "some-text\n{{ .vars.some_var }}\n",
		Vars: map[string]interface{}{
			"some_var": "some-templated-text",
		},
	}
	mockRenderer.On("RenderFile", expectedFileArgs).Return("some-rendered-template", nil)

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:       "some-id",
			Path:     fixturePath,
			Template: true,
			Vars: map[string]interface{}{
				"some_var": "some-templated-text",
			},
		},
	}, mockRenderer)
	assert.NoError(err)

	fileReader, fileSize, err := f.Read("some-id")
	assert.NoError(err)
	defer fileReader.Close()

	fileContents, err := ioutil.ReadAll(fileReader)
	assert.NoError(err)

	assert.Equal([]byte("some-rendered-template"), fileContents)
	assert.Equal(int64(len("some-rendered-template")), fileSize)

	mockRenderer.AssertExpectations(t)
}

func TestBuiltinRead(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)
	mockRenderer.On("RenderPath", "{{ builtin \"some-built-in\" }}").Return("__builtin__/installer/x86_64/kernel", nil)

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:   "some-id",
			Path: "{{ builtin \"some-built-in\" }}",
		},
	}, mockRenderer)
	assert.NoError(err)

	fileReader, fileSize, err := f.Read("some-id")
	assert.NoError(err)
	defer fileReader.Close()

	fileContents, err := ioutil.ReadAll(fileReader)
	assert.NoError(err)
	assert.NotEmpty(fileContents)
	assert.Greater(fileSize, int64(0))
}

func TestRemoteRead(t *testing.T) {
	assert := assert.New(t)

	assetsServer := httptest.NewServer(http.FileServer(http.Dir(fixturesDir())))
	defer assetsServer.Close()

	mockRenderer := new(MockRenderer)

	mockRenderer.On("RenderPath", mock.Anything).Return("", nil).Maybe()

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:  "some-id",
			URL: fmt.Sprintf("%s/files/simple.txt", assetsServer.URL),
		},
	}, mockRenderer)
	assert.NoError(err)

	fileReader, fileSize, err := f.Read("some-id")
	assert.NoError(err)
	defer fileReader.Close()

	fileContents, err := ioutil.ReadAll(fileReader)
	assert.NoError(err)

	assert.Equal([]byte("some-text\n"), fileContents)
	assert.Equal(int64(len("some-text\n")), fileSize)

	mockRenderer.AssertNumberOfCalls(t, "RenderFile", 0)
}

func TestErrorOnRemoteReadWithBadChecksum(t *testing.T) {
	assert := assert.New(t)

	assetsServer := httptest.NewServer(http.FileServer(http.Dir(fixturesDir())))
	defer assetsServer.Close()

	mockRenderer := new(MockRenderer)

	mockRenderer.On("RenderPath", mock.Anything).Return("", nil).Maybe()

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:     "some-id",
			URL:    fmt.Sprintf("%s/files/simple.txt", assetsServer.URL),
			SHA256: "1234",
		},
	}, mockRenderer)
	assert.NoError(err)

	_, _, err = f.Read("some-id")
	assert.NotNil(err)
	assert.Contains(err.Error(), "1234")
}

func TestReadErrorOnMissingFile(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)

	f, err := pxeserver.LoadFiles([]pxeserver.File{}, mockRenderer)
	assert.NoError(err)

	_, _, err = f.Read("some-missing-id")
	assert.NotNil(err)
	assert.Contains(err.Error(), "some-missing-id")
}

func TestReadErrorOnBadPath(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)
	fixturePath := path.Join(fixturesDir(), "files", "some-bad-path")

	mockRenderer.On("RenderPath", fixturePath).Return(fixturePath, nil)

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:   "some-id",
			Path: fixturePath,
		},
	}, mockRenderer)
	assert.NoError(err)

	_, _, err = f.Read("some-id")
	assert.NotNil(err)
	assert.Contains(err.Error(), "some-bad-path")
}

func TestReadErrorOnRendererFailure(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)
	fixturePath := path.Join(fixturesDir(), "files", "vars.txt")

	mockRenderer.On("RenderPath", fixturePath).Return(fixturePath, nil)
	mockRenderer.On("RenderFile", mock.Anything).Return("", errors.New("some-error"))

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:       "some-id",
			Path:     fixturePath,
			Template: true,
			Vars: map[string]interface{}{
				"some_var": "some-templated-text",
			},
		},
	}, mockRenderer)
	assert.NoError(err)

	_, _, err = f.Read("some-id")
	assert.NotNil(err)
	assert.Contains(err.Error(), "some-error")
}

func TestSimpleSHA256(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)
	fixturePath := path.Join(fixturesDir(), "files", "simple.txt")

	mockRenderer.On("RenderPath", fixturePath).Return(fixturePath, nil)

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:   "some-id",
			Path: fixturePath,
		},
	}, mockRenderer)
	assert.NoError(err)

	fileSHA, err := f.SHA256("some-id")
	assert.NoError(err)

	assert.Equal("58bfb70f49051a0b9c616ee59e5c979d7e704b822a18f84743703b14156548a9", fileSHA)

	mockRenderer.AssertNumberOfCalls(t, "RenderFile", 0)
}

func TestVarsSHA256(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)
	fixturePath := path.Join(fixturesDir(), "files", "vars.txt")

	mockRenderer.On("RenderPath", fixturePath).Return(fixturePath, nil)
	expectedFileArgs := pxeserver.RenderFileArgs{
		Template: "some-text\n{{ .vars.some_var }}\n",
		Vars: map[string]interface{}{
			"some_var": "some-templated-text",
		},
	}
	mockRenderer.On("RenderFile", expectedFileArgs).Return("some-rendered-template", nil)

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:       "some-id",
			Path:     fixturePath,
			Template: true,
			Vars: map[string]interface{}{
				"some_var": "some-templated-text",
			},
		},
	}, mockRenderer)
	assert.NoError(err)

	fileSHA, err := f.SHA256("some-id")
	assert.NoError(err)

	assert.Equal("01287663b74b37a4e4c318f5c12e675152188b4d324bf105c825b51d059ecf18", fileSHA)

	mockRenderer.AssertExpectations(t)
}

func TestSHA256ErrorOnMissingFile(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)

	f, err := pxeserver.LoadFiles([]pxeserver.File{}, mockRenderer)
	assert.NoError(err)

	_, err = f.SHA256("some-missing-id")
	assert.NotNil(err)
	assert.Contains(err.Error(), "some-missing-id")
}

func TestSimpleMD5(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)
	fixturePath := path.Join(fixturesDir(), "files", "simple.txt")

	mockRenderer.On("RenderPath", fixturePath).Return(fixturePath, nil)

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:   "some-id",
			Path: fixturePath,
		},
	}, mockRenderer)
	assert.NoError(err)

	fileMD5, err := f.MD5("some-id")
	assert.NoError(err)

	assert.Equal("e8f7eead7f5f754d970b4de0afa0cba9", fileMD5)

	mockRenderer.AssertNumberOfCalls(t, "RenderFile", 0)
}

func TestVarsMD5(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)
	fixturePath := path.Join(fixturesDir(), "files", "vars.txt")

	mockRenderer.On("RenderPath", fixturePath).Return(fixturePath, nil)
	expectedFileArgs := pxeserver.RenderFileArgs{
		Template: "some-text\n{{ .vars.some_var }}\n",
		Vars: map[string]interface{}{
			"some_var": "some-templated-text",
		},
	}
	mockRenderer.On("RenderFile", expectedFileArgs).Return("some-rendered-template", nil)

	f, err := pxeserver.LoadFiles([]pxeserver.File{
		{
			ID:       "some-id",
			Path:     fixturePath,
			Template: true,
			Vars: map[string]interface{}{
				"some_var": "some-templated-text",
			},
		},
	}, mockRenderer)
	assert.NoError(err)

	fileMD5, err := f.MD5("some-id")
	assert.NoError(err)

	assert.Equal("73ef82a56dc742e4db50667762f82701", fileMD5)

	mockRenderer.AssertExpectations(t)
}

func TestMD5ErrorOnMissingFile(t *testing.T) {
	assert := assert.New(t)

	mockRenderer := new(MockRenderer)

	f, err := pxeserver.LoadFiles([]pxeserver.File{}, mockRenderer)
	assert.NoError(err)

	_, err = f.MD5("some-missing-id")
	assert.NotNil(err)
	assert.Contains(err.Error(), "some-missing-id")
}
