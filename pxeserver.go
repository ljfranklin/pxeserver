package pxeserver

import (
	"io"
	"text/template"

	"go.universe.tf/netboot/pixiecore"
	"go.universe.tf/netboot/third_party/ipxe"
)

type Server struct {
	Config      io.Reader
	Address     string
	LogFunc     func(subsys, msg string)
	DHCPNoBind  bool
	SecretsPath string
}

func (s Server) Serve() error {
	// TODO: ipxe flags
	firmware := make(map[pixiecore.Firmware][]byte)
	firmware[pixiecore.FirmwareEFI64] = MustAsset("bindeps/ipxe/x86_64/ipxe.efi")
	firmware[pixiecore.FirmwareEFIBC] = MustAsset("bindeps/ipxe/x86_64/ipxe.efi")
	// TODO: compile this
	firmware[pixiecore.FirmwareX86Ipxe] = ipxe.MustAsset("ipxe.pxe")

	cfg, err := LoadConfig(s.Config)
	if err != nil {
		return err
	}
	var secrets Secrets
	if s.SecretsPath != "" {
		secrets, err = LoadLocalSecrets(s.SecretsPath, cfg.SecretDefs())
		if err != nil {
			return err
		}
	}
	renderer := Renderer{
		Secrets: secrets,
	}
	files, err := LoadFiles(cfg.Files(), renderer)
	if err != nil {
		return err
	}

	cmdlineTransform := func(tpl string, mac string, funcs template.FuncMap) (string, error) {
		vars, err := cfg.VarsForHost(mac)
		if err != nil {
			return "", err
		}
		return renderer.RenderCmdline(RenderCmdlineArgs{
			Template:   tpl,
			Mac:        mac,
			Vars:       vars,
			ExtraFuncs: funcs,
			Files:      files,
		})
	}

	booter, err := ConfigBooter(cfg.Pixiecore(), files)
	if err != nil {
		return err
	}

	server := &pixiecore.Server{
		Address:          s.Address,
		CmdlineTransform: cmdlineTransform,
		Booter:           booter,
		Ipxe:             firmware,
		Log:              s.LogFunc,
		DHCPNoBind:       s.DHCPNoBind,
	}
	return server.Serve()
}
