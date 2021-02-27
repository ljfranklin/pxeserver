// Copyright 2016 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pixiecore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"strconv"
	"strings"
	"text/template"

	"go.universe.tf/netboot/tftp"
)

func (s *Server) serveTFTP(l net.PacketConn) error {
	ts := tftp.Server{
		Handler:     s.handleTFTP,
		InfoLog:     func(msg string) { s.debug("TFTP", msg) },
		TransferLog: s.logTFTPTransfer,
	}
	err := ts.Serve(l)
	if err != nil {
		return fmt.Errorf("TFTP server shut down: %s", err)
	}
	return nil
}

type pathInfo struct {
	mac      net.HardwareAddr
	fwtype   int
	fileType string
	fileID   string
}

func extractInfo(path string) (pathInfo, error) {
	pathElements := strings.Split(path, "/")
	if len(pathElements) < 2 {
		return pathInfo{}, errors.New("not found")
	}

	mac, err := net.ParseMAC(pathElements[0])
	if err != nil {
		return pathInfo{}, fmt.Errorf("invalid MAC address %q", pathElements[0])
	}

	i, err := strconv.Atoi(pathElements[1])
	if err != nil {
		return pathInfo{}, errors.New("not found")
	}

	fileType := ""
	if len(pathElements) > 2 {
		fileType = pathElements[2]
	}
	fileID := ""
	if len(pathElements) > 3 {
		fileID = pathElements[3]
	}

	return pathInfo{
		mac:      mac,
		fwtype:   i,
		fileType: fileType,
		fileID:   fileID,
	}, nil
}

func (s *Server) logTFTPTransfer(clientAddr net.Addr, path string, err error) {
	info, pathErr := extractInfo(path)
	if pathErr != nil {
		s.log("TFTP", "unable to extract mac from request:%v", pathErr)
		return
	}
	if err != nil {
		s.log("TFTP", "Send of %q to %s failed: %s", path, clientAddr, err)
	} else {
		s.log("TFTP", "Sent %q to %s", path, clientAddr)
		s.machineEvent(info.mac, machineStateTFTP, "Sent iPXE to %s", clientAddr)
	}
}

func (s *Server) handleTFTP(path string, clientAddr net.Addr, serverIP string) (io.ReadCloser, int64, error) {
	info, err := extractInfo(path)
	if err != nil {
		return nil, 0, fmt.Errorf("unknown path %q", path)
	}

	if info.fileType == "empty" {
		return ioutil.NopCloser(&bytes.Buffer{}), int64(0), nil
	}
	if info.fileType == "pxelinux.cfg" && info.fileID == "default" {
		return s.handlePXELinux(serverIP, info)
	}
	if len(info.fileID) > 0 {
		// TODO(ljfranklin): log 'kernel send'
		return s.Booter.ReadBootFile(ID(info.fileID))
	}

	bs, ok := s.Ipxe[Firmware(info.fwtype)]
	if !ok {
		return nil, 0, fmt.Errorf("unknown firmware type %d", info.fwtype)
	}

	return ioutil.NopCloser(bytes.NewBuffer(bs)), int64(len(bs)), nil
}

func (s *Server) handlePXELinux(serverIP string, info pathInfo) (io.ReadCloser, int64, error) {
	arch, err := fwtypeToArch(Firmware(info.fwtype))
	if err != nil {
		return nil, 0, err
	}
	mach := Machine{
		MAC:  info.mac,
		Arch: arch,
	}
	spec, err := s.Booter.BootSpec(mach)
	if err != nil {
		return nil, 0, err
	}

	var b bytes.Buffer
	b.WriteString(`DEFAULT default
LABEL default
`)
	b.WriteString(fmt.Sprintf("\tkernel kernel/%s\n", spec.Kernel))
	if len(spec.Initrd) > 0 {
		initrdPaths := make([]string, 0, len(spec.Initrd))
		for _, initrd := range spec.Initrd {
			initrdPaths = append(initrdPaths, fmt.Sprintf("initrd/%s", initrd))
		}
		b.WriteString(fmt.Sprintf("\tinitrd %s\n", strings.Join(initrdPaths, ",")))
	}
	if len(spec.Cmdline) > 0 {
		serverHost := fmt.Sprintf("%s:%d", serverIP, s.HTTPPort)
		f := func(id string) string {
			return fmt.Sprintf("http://%s/_/file?name=%s", serverHost, url.QueryEscape(id))
		}
		cmdline, err := s.CmdlineTransform(spec.Cmdline, info.mac.String(), template.FuncMap{"ID": f})
		if err != nil {
			return nil, 0, fmt.Errorf("expanding cmdline %q: %s", spec.Cmdline, err)
		}
		b.WriteString(fmt.Sprintf("\tappend %s\n", cmdline))
	}
	b.WriteString(`
PROMPT 1
TIMEOUT 0
`)

	return ioutil.NopCloser(&b), int64(b.Len()), nil
}

func fwtypeToArch(fwtype Firmware) (Architecture, error) {
	switch fwtype {
	case FirmwareX86PC, FirmwareEFI32, FirmwareX86Ipxe:
		return ArchIA32, nil
	case FirmwareEFI64, FirmwareEFIBC, FirmwarePixiecoreIpxe:
		return ArchX64, nil
	case FirmwareEFIArm32:
		return ArchArm32, nil
	case FirmwareEFIArm64:
		return ArchArm64, nil
	}
	return 0, fmt.Errorf("failed to lookup arch for firmware type '%d' (please file a bug!)", fwtype)
}
