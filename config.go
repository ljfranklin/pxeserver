package pxeserver

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	// This yaml library outputs maps with string keys for better
	// interoperability with template funcs like 'toJson'
	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
)

type Config struct {
	macToFiles      map[string][]File
	macToVars       map[string]map[string]interface{}
	macToSecrets    map[string][]SecretDef
	pixiecoreConfig Pixiecore
}

type ServerConfig struct {
	Hosts []Host
	Vars  map[string]interface{}
}
type Pixiecore map[MacAddress]MachineConfig
type MacAddress string
type MachineConfig struct {
	Kernel  string
	Initrd  []string
	Cmdline string
	ForcePXELinux bool
}
type Host struct {
	Mac           string
	Kernel        File
	Initrds       []File
	Files         []File
	BootArgs      []string `json:"boot_args"`
	Vars          map[string]interface{}
	Secrets       []SecretDef
	ForcePXELinux bool `json:"force_pxe_linux"`
}
type File struct {
	Mac          string
	Path         string
	URL          string
	SHA256       string
	ID           string
	Template     bool
	Vars         map[string]interface{}
	ImageConvert ImageConvert `json:"image_convert"`
	Gzip         bool
}
type ImageConvert struct {
	InputFormat string `json:"input_format"`
}

func LoadConfig(configReader io.Reader) (Config, error) {
	c := Config{
		pixiecoreConfig: Pixiecore{},
		macToFiles:      make(map[string][]File),
		macToVars:       make(map[string]map[string]interface{}),
		macToSecrets:    make(map[string][]SecretDef),
	}

	input := ServerConfig{}
	configContents, err := ioutil.ReadAll(configReader)
	if err != nil {
		return Config{}, err
	}
	if err = yaml.Unmarshal(configContents, &input); err != nil {
		return Config{}, fmt.Errorf("config file was not valid YAML/JSON: %s", err)
	}

	for _, host := range input.Hosts {
		machine := MachineConfig{}

		host.Kernel.ID = fmt.Sprintf("%s-__kernel__", host.Mac)
		host.Kernel.Mac = host.Mac
		c.macToFiles[host.Mac] = append(c.macToFiles[host.Mac], host.Kernel)
		machine.Kernel = host.Kernel.ID

		for i, f := range host.Initrds {
			f.ID = fmt.Sprintf("%s-__initrd%d__", host.Mac, i)
			f.Mac = host.Mac
			c.macToFiles[host.Mac] = append(c.macToFiles[host.Mac], f)
			machine.Initrd = append(machine.Initrd, f.ID)
		}

		if err := mergo.Merge(&host.Vars, input.Vars); err != nil {
			return Config{}, err
		}

		c.macToVars[host.Mac] = host.Vars
		c.macToSecrets[host.Mac] = host.Secrets

		for _, f := range host.Files {
			if len(f.Vars) > 0 && !f.Template {
				return Config{}, fmt.Errorf("file with ID '%s' must have 'template: true' if 'vars' are non-empty", f.ID)
			}

			if err := mergo.Merge(&f.Vars, host.Vars, mergo.WithOverride); err != nil {
				return Config{}, err
			}
			f.ID = fmt.Sprintf("%s-%s", host.Mac, f.ID)
			f.Mac = host.Mac
			c.macToFiles[host.Mac] = append(c.macToFiles[host.Mac], f)
		}

		machine.Cmdline = strings.Join(host.BootArgs, " ")
		machine.ForcePXELinux = host.ForcePXELinux

		c.pixiecoreConfig[MacAddress(host.Mac)] = machine
	}

	return c, nil
}

func (c *Config) Pixiecore() Pixiecore {
	return c.pixiecoreConfig
}

func (c *Config) Files() []File {
	allFiles := []File{}
	for _, filesForHost := range c.macToFiles {
		allFiles = append(allFiles, filesForHost...)
	}
	return allFiles
}

func (c *Config) SecretDefs() map[string][]SecretDef {
	return c.macToSecrets
}

func (c *Config) VarsForHost(mac string) (map[string]interface{}, error) {
	vars, ok := c.macToVars[mac]
	if !ok {
		return nil, fmt.Errorf("could not find host '%s' in config file", mac)
	}
	return vars, nil
}
