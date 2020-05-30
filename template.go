package pxeserver

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	// This yaml library outputs maps with string keys for better
	// interoperability with template funcs like 'toJson'
	"github.com/ghodss/yaml"
)

type Renderer struct {}

type RenderFileArgs struct {
	Template string
	Vars map[string]interface{}
}

func (r Renderer) RenderFile(args RenderFileArgs) (string, error) {
	// File helper funcs are only available inside Cmdline templates
	noopValue := "<no value>"
	getFileURL := func(id string) (string, error) {
		return noopValue, nil
	}
	getFileSHA256 := func(id string) (string, error) {
		return noopValue, nil
	}
	getFileMD5 := func(id string) (string, error) {
		return noopValue, nil
	}
	templateFuncs := template.FuncMap{
		"file_url":    getFileURL,
		"file_sha256": getFileSHA256,
		"file_md5":    getFileMD5,
	}

	vars, err := r.templateVars(args.Vars, templateFuncs)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("file").
	  Funcs(sprig.TxtFuncMap()).
		Option("missingkey=error").
		Parse(string(args.Template))
	if err != nil {
		return "", err
	}
	var templatedReader bytes.Buffer
	if err = tmpl.Execute(&templatedReader, map[string]interface{}{"vars": vars}); err != nil {
		return "", err
	}
	return templatedReader.String(), nil
}

type fileHelper interface {
  SHA256(id string) (string, error)
  MD5(id string) (string, error)
}

type RenderCmdlineArgs struct {
	Template string
	Mac string
	Vars map[string]interface{}
	ExtraFuncs template.FuncMap
	Files fileHelper
}

func (r Renderer) RenderCmdline(args RenderCmdlineArgs) (string, error) {
	getFileURL := func(id string) (string, error) {
		namespacedID := fmt.Sprintf("%s-%s", args.Mac, id)
		idFunc := args.ExtraFuncs["ID"].(func(string) string)
		return idFunc(namespacedID), nil
	}
	getFileSHA256 := func(id string) (string, error) {
		return args.Files.SHA256(fmt.Sprintf("%s-%s", args.Mac, id))
	}
	getFileMD5 := func(id string) (string, error) {
		return args.Files.MD5(fmt.Sprintf("%s-%s", args.Mac, id))
	}
	templateFuncs := template.FuncMap{
		"file_url":    getFileURL,
		"file_sha256": getFileSHA256,
		"file_md5":    getFileMD5,
	}

	vars, err := r.templateVars(args.Vars, templateFuncs)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("cmdline").
	  Funcs(templateFuncs).
		Funcs(sprig.TxtFuncMap()).
		Option("missingkey=error").
		Parse(args.Template)
	if err != nil {
		return "", err
	}
	var templatedCmdline bytes.Buffer
	if err = tmpl.Execute(&templatedCmdline, map[string]interface{}{"vars": vars}); err != nil {
		return "", err
	}

	return templatedCmdline.String(), nil
}

func (r Renderer) RenderPath(filepath string) (string, error) {
	getBuiltin := func(builtinPath string) (string, error) {
		return fmt.Sprintf("__builtin__/%s", builtinPath), nil
	}
	templateFuncs := template.FuncMap{
		"builtin":    getBuiltin,
	}

	tmpl, err := template.New("path").
	  Funcs(templateFuncs).
		Option("missingkey=error").
		Parse(filepath)
	if err != nil {
		return "", err
	}
	var templatedPath bytes.Buffer
	if err = tmpl.Execute(&templatedPath, nil); err != nil {
		return "", err
	}

	return templatedPath.String(), nil
}

func (r Renderer) templateVars(vars map[string]interface{}, funcs template.FuncMap) (map[string]interface{}, error){
	varsYAML, err := yaml.Marshal(vars)
	if err != nil {
		return nil, err
	}

	varsTmpl, err := template.New("vars").Funcs(funcs).Funcs(sprig.TxtFuncMap()).Option("missingkey=error").Parse(string(varsYAML))
	if err != nil {
		return nil, err
	}
	var templatedVars bytes.Buffer
	if err = varsTmpl.Execute(&templatedVars, nil); err != nil {
		return nil, err
	}
	var processedVars map[string]interface{}
	if err = yaml.Unmarshal(templatedVars.Bytes(), &processedVars); err != nil {
		return nil, err
	}

	return processedVars, nil
}
