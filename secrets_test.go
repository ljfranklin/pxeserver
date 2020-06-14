package pxeserver_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/ljfranklin/pxeserver"
	"github.com/stretchr/testify/assert"
)

// TODO: test global secrets
func TestExistingSecretsForHost(t *testing.T) {
	assert := assert.New(t)

	existingSecrets := path.Join(fixturesDir(), "secrets", "secrets.yaml")
	secretsCfg, err := pxeserver.LoadLocalSecrets(existingSecrets, nil)
	assert.NoError(err)

	secret, err := secretsCfg.Get("52:54:00:12:34:56", "/some_namespace/some_var")
	assert.NoError(err)
	assert.Equal(secret, "some_value")
}

func TestGeneratedSecret(t *testing.T) {
	assert := assert.New(t)

	defs := make(map[string][]pxeserver.SecretDef)
	for i := 0; i < 10; i++ {
		host := fmt.Sprintf("some-mac-%d", i)
		defs[host] = []pxeserver.SecretDef{
			{
				ID:   "/some_namespace/some_var",
				Type: "password",
			},
		}
	}

	tmpdir, err := ioutil.TempDir("", "pxeserver-secrets")
	assert.NoError(err)
	defer os.RemoveAll(tmpdir)
	secretsPath := path.Join(tmpdir, "secrets.yaml")
	secretsCfg, err := pxeserver.LoadLocalSecrets(secretsPath, defs)
	assert.NoError(err)

	// ensure we get different passwords on each call
	seenPasswords := map[string]string{}
	for i := 0; i < 10; i++ {
		host := fmt.Sprintf("some-mac-%d", i)
		secret, err := secretsCfg.GetOrGenerate(host, "/some_namespace/some_var")
		assert.NoError(err)
		assert.Len(secret, 20)

		duplicatePassword := false
		for _, v := range seenPasswords {
			if v == secret.(string) {
				duplicatePassword = true
				break
			}
		}
		assert.False(duplicatePassword)

		seenPasswords[host] = secret.(string)
	}

	// reload config to ensure changes are persisted
	secretsCfg, err = pxeserver.LoadLocalSecrets(secretsPath, nil)
	assert.NoError(err)
	for i := 0; i < 10; i++ {
		host := fmt.Sprintf("some-mac-%d", i)
		secret, err := secretsCfg.Get(host, "/some_namespace/some_var")
		assert.NoError(err)
		assert.Equal(secret, seenPasswords[host])
	}
}

func TestGeneratedSshKey(t *testing.T) {
	assert := assert.New(t)

	defs := map[string][]pxeserver.SecretDef{
		"some-host": {
			{
				ID:   "/some_namespace/some_var",
				Type: "ssh_key",
				Opts: map[string]interface{}{
					"comment": "some-user",
				},
			},
		},
	}

	emptySecrets, err := ioutil.TempFile("", "pxeserver-secrets")
	assert.NoError(err)
	defer os.Remove(emptySecrets.Name())
	secretsCfg, err := pxeserver.LoadLocalSecrets(emptySecrets.Name(), defs)
	assert.NoError(err)

	secret, err := secretsCfg.GetOrGenerate("some-host", "/some_namespace/some_var")
	assert.NoError(err)
	privateKey := secret.(map[string]interface{})["private_key"]
	publicKey := secret.(map[string]interface{})["public_key"]
	assert.Contains(privateKey, "BEGIN RSA PRIVATE KEY")
	assert.Regexp("^ssh-rsa .+ some-user\n$", publicKey)
}

func TestGetField(t *testing.T) {
	assert := assert.New(t)

	existingSecrets := path.Join(fixturesDir(), "secrets", "secrets-map.yaml")
	secretsCfg, err := pxeserver.LoadLocalSecrets(existingSecrets, nil)
	assert.NoError(err)

	secret, err := secretsCfg.GetField("some-host", "/some_namespace/some_var", "some_field")
	assert.NoError(err)
	assert.Equal(secret, "some_value")
}

func TestLocalErrorOnMissingSecretsHost(t *testing.T) {
	assert := assert.New(t)

	existingSecrets := path.Join(fixturesDir(), "secrets", "secrets.yaml")
	secretsCfg, err := pxeserver.LoadLocalSecrets(existingSecrets, nil)
	assert.NoError(err)

	_, err = secretsCfg.Get("missing-host", "/some_namespace/some_var")
	assert.NotNil(err)
	assert.Contains(err.Error(), "missing-host")
}

func TestLocalErrorOnMissingSecretsID(t *testing.T) {
	assert := assert.New(t)

	existingSecrets := path.Join(fixturesDir(), "secrets", "secrets.yaml")
	secretsCfg, err := pxeserver.LoadLocalSecrets(existingSecrets, nil)
	assert.NoError(err)

	_, err = secretsCfg.Get("52:54:00:12:34:56", "missing-id")
	assert.NotNil(err)
	assert.Contains(err.Error(), "missing-id")
}
