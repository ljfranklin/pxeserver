package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/onsi/gomega/gexec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationBoot(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	testToggle := os.Getenv("PXESERVER_TEST_INTEGRATION")
	require.Equal(testToggle, "true", "PXESERVER_TEST_INTEGRATION not set; run integration tests with ./scripts/test_integration")

	tmpdir, err := ioutil.TempDir("", "pxeserver-test")
	require.NoError(err)
	defer os.RemoveAll(tmpdir)

	logger := logWriter{t: t}

	binaryPath, err := gexec.Build("github.com/ljfranklin/pxeserver/cli/pxeserver")
	require.NoError(err)
	defer gexec.CleanupBuildArtifacts()

	err = os.Chdir(testDir())
	require.NoError(err)

	configPath := path.Join("fixtures", "config.yaml")
	secretsPath := path.Join(tmpdir, "secrets.yaml")

	diskPath := path.Join(tmpdir, "disk.img")
	err = createEmptyDiskImage(diskPath, &logger)
	require.NoError(err)

	pxeCxt, pxeCancel := context.WithCancel(context.Background())
	pxeCmd := exec.CommandContext(pxeCxt, binaryPath,
		"boot",
		fmt.Sprintf("--config=%s", configPath),
		fmt.Sprintf("--secrets=%s", secretsPath),
	)
	pxeCmd.Stdout = &logger
	pxeCmd.Stderr = &logger
	err = pxeCmd.Start()
	require.NoError(err)

	t.Log("Starting QEMU, view output with 'vncviewer -Shared localhost:5910'...")
	runQemuPath := fmt.Sprintf("DISK_PATH=%s %s", diskPath, path.Join(testDir(), "run_qemu.sh"))
	qemuNetbootCxt, qemuNetbootCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer qemuNetbootCancel()
	qemuCmd := exec.CommandContext(qemuNetbootCxt, "/bin/bash", "-c", runQemuPath)
	qemuCmd.Stdout = &logger
	qemuCmd.Stderr = &logger
	err = qemuCmd.Run()
	assert.NoError(err)

	pxeCancel()
	_ = pxeCmd.Wait()

	// TODO: can flag format be simplified?
	pxeCmd = exec.Command(binaryPath,
		"secrets",
		fmt.Sprintf("--secrets=%s", secretsPath),
		"--host=52:54:00:12:34:56",
		"--id=/cloud_init/users/boot-test/ssh_key",
		"--field=private_key",
	)
	sshKey, err := pxeCmd.Output()
	if !assert.NoError(err) {
		require.FailNow(string(err.(*exec.ExitError).Stderr))
	}
	signer, err := ssh.ParsePrivateKey(sshKey)
	require.NoError(err)

	t.Log("Starting QEMU with disk boot, view output with 'vncviewer -Shared localhost:5910'...")
	qemuDiskBootCxt, qemuDiskBootCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer qemuDiskBootCancel()
	qemuDiskBootCmd := exec.CommandContext(qemuDiskBootCxt, "/bin/bash", "-c", runQemuPath)
	qemuDiskBootCmd.Stdout = &logger
	qemuDiskBootCmd.Stderr = &logger
	err = qemuDiskBootCmd.Start()
	assert.NoError(err)

	config := &ssh.ClientConfig{
		User: "boot-test",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Minute,
	}

	t.Log("Attempting to open SSH connection...")
	var client *ssh.Client
	var sshErr error
	for i := 0; i < 30; i++ {
		client, sshErr = ssh.Dial("tcp", "172.20.0.10:22", config)
		if sshErr == nil {
			break
		}
		t.Logf("Got ssh error '%s', retrying...", sshErr.Error())
		time.Sleep(10 * time.Second)
	}
	require.NoError(sshErr)

	t.Logf("Got successful ssh connection, making assertions...")
	session, err := client.NewSession()
	require.NoError(err)
	var hostnameOutput bytes.Buffer
	session.Stdout = &hostnameOutput
	session.Stderr = &logger
	err = session.Run("hostname")
	assert.NoError(err)
	_ = session.Close()

	assert.Equal("boot-test", strings.TrimSpace(hostnameOutput.String()))

	t.Logf("Done, killing qemu...")
	qemuDiskBootCmd.Process.Signal(os.Interrupt)
	_ = qemuDiskBootCmd.Wait()
}

type logWriter struct {
	t *testing.T
}

func (l logWriter) Write(p []byte) (int, error) {
	l.t.Log(string(p))
	return len(p), nil
}

func testDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return path.Dir(filename)
}

func waitWithTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	errChan := make(chan error)
	go func() {
		errChan <- cmd.Wait()
	}()
	select {
	case waitErr := <-errChan:
		return waitErr
	case <-time.After(timeout):
		return fmt.Errorf("Command failed to finish after %s", timeout)
	}
}

func createEmptyDiskImage(path string, logger io.Writer) error {
	pxeCmd := exec.Command("qemu-img", "create", path, "4G")
	pxeCmd.Stdout = logger
	pxeCmd.Stderr = logger
	return pxeCmd.Run()
}
