package pxeserver

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type Files struct {
	// TODO: rename to ConfigFile?
	availableFiles map[string]File
	renderer       renderer
}

type renderer interface {
	RenderFile(RenderFileArgs) (string, error)
	RenderPath(string) (string, error)
}

func LoadFiles(files []File, renderer renderer) (Files, error) {
	f := Files{
		availableFiles: make(map[string]File),
		renderer:       renderer,
	}
	for _, cfgFile := range files {
		var err error
		cfgFile.Path, err = renderer.RenderPath(cfgFile.Path)
		if err != nil {
			return Files{}, err
		}
		f.availableFiles[cfgFile.ID] = cfgFile
	}
	return f, nil
}

func (f Files) SHA256(id string) (string, error) {
	inputFile, _, err := f.Read(id)
	if err != nil {
		return "", err
	}
	defer inputFile.Close()

	checksumWriter := sha256.New()
	if _, err := io.Copy(checksumWriter, inputFile); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", checksumWriter.Sum(nil)), nil
}

func (f Files) MD5(id string) (string, error) {
	inputFile, _, err := f.Read(id)
	if err != nil {
		return "", err
	}
	defer inputFile.Close()

	checksumWriter := md5.New()
	if _, err := io.Copy(checksumWriter, inputFile); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", checksumWriter.Sum(nil)), nil
}

func (f Files) Read(id string) (io.ReadCloser, int64, error) {
	file, ok := f.availableFiles[id]
	if !ok {
		return nil, -1, fmt.Errorf("Could not find file with ID '%s'", id)
	}

	var fileReader io.ReadCloser
	var fileSize int64
	var fileErr error
	if strings.HasPrefix(file.Path, "__builtin__") {
		fileReader, fileSize, fileErr = f.readBuiltinFile(file)
	} else if file.URL != "" {
		fileReader, fileSize, fileErr = f.readRemoteFile(file)
	} else {
		fileReader, fileSize, fileErr = f.readLocalFile(file)
	}
	if fileErr != nil {
		return nil, -1, fileErr
	}

	if file.ImageConvert.InputFormat != "" {
		fileReader, fileSize, fileErr = f.convertQcowToRaw(fileReader)
	}

	if file.Gzip {
		return f.gzip(fileReader)
	}
	return fileReader, fileSize, fileErr
}

func (f Files) readLocalFile(file File) (io.ReadCloser, int64, error) {
	inputFile, err := os.Open(file.Path)
	if err != nil {
		return nil, -1, err
	}
	if !file.Template {
		stat, err := inputFile.Stat()
		if err != nil {
			return nil, -1, err
		}
		return inputFile, stat.Size(), nil
	}

	templateContent, err := ioutil.ReadAll(inputFile)
	if err != nil {
		return nil, -1, err
	}
	inputFile.Close()

	rendererContent, err := f.renderer.RenderFile(RenderFileArgs{
		Mac:      file.Mac,
		Template: string(templateContent),
		Vars:     file.Vars,
	})
	if err != nil {
		return nil, -1, err
	}

	return ioutil.NopCloser(strings.NewReader(rendererContent)), int64(len(rendererContent)), nil
}

type readCloserWithDelete struct {
	file *os.File
}

func (r readCloserWithDelete) Read(p []byte) (int, error) {
	return r.file.Read(p)
}
func (r readCloserWithDelete) Close() error {
	r.file.Close()
	return os.Remove(r.file.Name())
}

func (f Files) readRemoteFile(file File) (io.ReadCloser, int64, error) {
	resp, err := http.Get(file.URL)
	if err != nil {
		return nil, -1, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		// TODO: print body on error
		return nil, -1, fmt.Errorf("bad status '%d' when downloading remote file '%s'", resp.StatusCode, file.URL)
	}

	// TODO: are these file permissions okay?
	tmpfile, err := ioutil.TempFile("", "pxeserver")
	if err != nil {
		return nil, -1, err
	}

	hasher := sha256.New()
	var teeReader io.Reader
	if file.SHA256 != "" {
		teeReader = io.TeeReader(resp.Body, hasher)
	} else {
		teeReader = resp.Body
	}

	_, err = io.Copy(tmpfile, teeReader)
	if err != nil {
		return nil, -1, err
	}
	_, err = tmpfile.Seek(0, 0)
	if err != nil {
		return nil, -1, err
	}

	if file.SHA256 != "" {
		actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
		if actualHash != file.SHA256 {
			return nil, -1, fmt.Errorf("expected '%s' to have checksum '%s' but was '%s'", file.ID, file.SHA256, actualHash)
		}
	}

	stat, err := tmpfile.Stat()
	if err != nil {
		return nil, -1, err
	}
	// TODO: use renderer if template

	return readCloserWithDelete{
		file: tmpfile,
	}, stat.Size(), nil
}

func (f Files) readBuiltinFile(file File) (io.ReadCloser, int64, error) {
	builtinPath := strings.Replace(file.Path, "__builtin__", "bindeps", 1)
	data, err := Asset(builtinPath)
	if err != nil {
		return nil, -1, err
	}
	return ioutil.NopCloser(bytes.NewReader(data)), int64(len(data)), nil
}

func (f Files) convertQcowToRaw(qcowReader io.ReadCloser) (io.ReadCloser, int64, error) {
	// TODO: are these file permissions okay?
	inputFile, err := ioutil.TempFile("", "pxeserver-qcow")
	if err != nil {
		return nil, -1, err
	}
	_, err = io.Copy(inputFile, qcowReader)
	if err != nil {
		return nil, -1, err
	}
	err = qcowReader.Close()
	if err != nil {
		return nil, -1, err
	}
	err = inputFile.Close()
	if err != nil {
		return nil, -1, err
	}

	outputFile, err := ioutil.TempFile("", "pxeserver-raw")
	if err != nil {
		return nil, -1, err
	}
	convertCmd := exec.Command("qemu-img", "convert",
		"-f", "qcow2", "-O", "raw", inputFile.Name(), outputFile.Name())
	// TODO: pass in logger
	convertCmd.Stdout = os.Stderr
	convertCmd.Stderr = os.Stderr
	err = convertCmd.Run()
	if err != nil {
		return nil, -1, err
	}

	stat, err := outputFile.Stat()
	if err != nil {
		return nil, -1, err
	}

	return readCloserWithDelete{
		file: outputFile,
	}, stat.Size(), nil
}

func (f Files) gzip(inputReader io.ReadCloser) (io.ReadCloser, int64, error) {
	defer inputReader.Close()

	outputFile, err := ioutil.TempFile("", "pxeserver-gzip")
	if err != nil {
		return nil, -1, err
	}

	gzipWriter := gzip.NewWriter(outputFile)

	_, err = io.Copy(gzipWriter, inputReader)
	if err != nil {
		return nil, -1, err
	}
	err = gzipWriter.Close()
	if err != nil {
		return nil, -1, err
	}
	_, err = outputFile.Seek(0, 0)
	if err != nil {
		return nil, -1, err
	}

	stat, err := outputFile.Stat()
	if err != nil {
		return nil, -1, err
	}

	return readCloserWithDelete{
		file: outputFile,
	}, stat.Size(), nil
}
