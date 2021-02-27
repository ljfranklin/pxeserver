package pxeserver

import (
	"fmt"
	"io"

	"go.universe.tf/netboot/pixiecore"
)

type configBooter struct {
	specs map[string]*pixiecore.Spec
	files Files
}

func ConfigBooter(cfg Pixiecore, files Files) (pixiecore.Booter, error) {
	ret := &configBooter{
		specs: make(map[string]*pixiecore.Spec),
		files: files,
	}

	for mac, hostCfg := range cfg {
		spec := &pixiecore.Spec{
			Kernel: pixiecore.ID(hostCfg.Kernel),
		}
		for _, initrd := range hostCfg.Initrd {
			spec.Initrd = append(spec.Initrd, pixiecore.ID(initrd))
		}
		spec.Cmdline = hostCfg.Cmdline
		spec.ForcePXELinux = hostCfg.ForcePXELinux

		ret.specs[string(mac)] = spec
	}

	return ret, nil
}

func (s *configBooter) BootSpec(m pixiecore.Machine) (*pixiecore.Spec, error) {
	mac := m.MAC.String()
	spec, ok := s.specs[mac]
	if !ok {
		return nil, fmt.Errorf("Could not find BootSpec for '%s'", mac)
	}
	return spec, nil
}

func (s *configBooter) ReadBootFile(id pixiecore.ID) (io.ReadCloser, int64, error) {
	return s.files.Read(string(id))
}

// unused
func (s *configBooter) WriteBootFile(pixiecore.ID, io.Reader) error {
	return nil
}
