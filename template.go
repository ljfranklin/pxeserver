package pxeserver

import (
	"bytes"
	"fmt"
	"reflect"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

type Renderer struct {
	Secrets Secrets
}

type RenderFileArgs struct {
	Mac      string
	Template string
	Vars     map[string]interface{}
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
	getSecret := func(id string) (interface{}, error) {
		return r.Secrets.GetOrGenerate(args.Mac, id)
	}
	getSharedSecret := func(id string) (interface{}, error) {
		return r.Secrets.GetOrGenerate("", id)
	}
	templateFuncs := template.FuncMap{
		"file_url":      getFileURL,
		"file_sha256":   getFileSHA256,
		"file_md5":      getFileMD5,
		"secret":        getSecret,
		"shared_secret": getSharedSecret,
	}

	vars, err := r.templateVars(args.Vars, templateFuncs)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("file").
		Funcs(templateFuncs).
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
	Template   string
	Mac        string
	Vars       map[string]interface{}
	ExtraFuncs template.FuncMap
	Files      fileHelper
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
	getSecret := func(id string) (interface{}, error) {
		return r.Secrets.GetOrGenerate(args.Mac, id)
	}
	getSharedSecret := func(id string) (interface{}, error) {
		return r.Secrets.GetOrGenerate("", id)
	}
	templateFuncs := template.FuncMap{
		"file_url":      getFileURL,
		"file_sha256":   getFileSHA256,
		"file_md5":      getFileMD5,
		"secret":        getSecret,
		"shared_secret": getSharedSecret,
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
		"builtin": getBuiltin,
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

func (r Renderer) templateVars(vars map[string]interface{}, funcs template.FuncMap) (map[string]interface{}, error) {
	result, err := r.templateSingleVar(vars, funcs)
	if err != nil {
		return nil, err
	}
	return result.(map[string]interface{}), nil
}

func (r Renderer) templateSingleVar(value interface{}, funcs template.FuncMap) (interface{}, error) {
	switch v := reflect.ValueOf(value); v.Kind() {
	case reflect.Map:
		templatedMap := make(map[string]interface{})
		mapIter := v.MapRange()
		for mapIter.Next() {
			k := mapIter.Key()
			v := mapIter.Value()
			templatedKey, err := r.templateString(k.String(), funcs)
			if err != nil {
				return nil, err
			}
			templatedValue, err := r.templateSingleVar(v.Interface(), funcs)
			if err != nil {
				return nil, err
			}
			templatedMap[templatedKey] = templatedValue
		}
		return templatedMap, nil
	case reflect.Slice:
		templatedSlice := make([]interface{}, 0)
		for i := 0; i < v.Len(); i++ {
			v := v.Index(i)
			templatedValue, err := r.templateSingleVar(v.Interface(), funcs)
			if err != nil {
				return nil, err
			}
			templatedSlice = append(templatedSlice, templatedValue)
		}
		return templatedSlice, nil
	case reflect.String:
		return r.templateString(v.String(), funcs)
	default:
		return value, nil
	}
}

func (r Renderer) templateString(s string, funcs template.FuncMap) (string, error) {
	varsTmpl, err := template.New("vars").Funcs(funcs).Funcs(sprig.TxtFuncMap()).Option("missingkey=error").Parse(s)
	if err != nil {
		return "", err
	}
	var templatedVars bytes.Buffer
	if err = varsTmpl.Execute(&templatedVars, nil); err != nil {
		return "", err
	}
	return templatedVars.String(), nil
}
