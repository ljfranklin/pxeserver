package pxeserver

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"strings"
	"sync"

	yamlToJson "github.com/ghodss/yaml"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

const (
	letters  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits   = "0123456789"
	symbols  = " !\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	allChars = letters + digits + symbols
)

type Secrets interface {
	GetOrGenerate(mac string, id string) (interface{}, error)
	Get(mac string, id string) (interface{}, error)
	GetField(mac string, id string, field string) (interface{}, error)
}

type localSecrets struct {
	storePath     string
	hostToSecrets map[string]map[string]interface{}
	hostToDefs    map[string][]SecretDef
	mu            sync.Mutex
}

type secretsConfig struct {
	Hosts []hostSecrets
}

type hostSecrets struct {
	Mac     string
	Secrets []secret
}

type secret struct {
	ID    string
	Value interface{}
}
type SecretDef struct {
	ID   string
	Type string
	Opts map[string]interface{}
}

func LoadLocalSecrets(storePath string, hostToDefs map[string][]SecretDef) (Secrets, error) {
	secrets := localSecrets{
		hostToSecrets: make(map[string]map[string]interface{}),
		hostToDefs:    hostToDefs,
		storePath:     storePath,
	}

	_, err := os.Stat(storePath)
	if err == nil {
		configContents, err := ioutil.ReadFile(storePath)
		if err != nil {
			return nil, err
		}

		var initialConfig secretsConfig
		if err = yamlToJson.Unmarshal(configContents, &initialConfig); err != nil {
			return nil, err
		}

		for _, host := range initialConfig.Hosts {
			secrets.hostToSecrets[host.Mac] = make(map[string]interface{})
			for _, s := range host.Secrets {
				secrets.hostToSecrets[host.Mac][s.ID] = s.Value
			}
		}
	}

	return &secrets, nil
}

func (s *localSecrets) GetOrGenerate(mac string, id string) (interface{}, error) {
	if secret, err := s.Get(mac, id); err == nil {
		return secret, nil
	}

	s.mu.Lock()
	hostDefs, ok := s.hostToDefs[mac]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("could not find secret defs for host '%s'", mac)
	}
	if err := s.generate(mac, hostDefs); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	s.mu.Unlock()
	return s.Get(mac, id)
}

func (s *localSecrets) Get(mac string, id string) (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hostSecrets, ok := s.hostToSecrets[mac]
	if !ok {
		return nil, fmt.Errorf("could not find secrets for host '%s'", mac)
	}
	secret, ok := hostSecrets[id]
	if !ok {
		return nil, fmt.Errorf("could not find secret with id '%s' for host '%s'", id, mac)
	}
	return secret, nil
}

func (s *localSecrets) GetField(mac string, id string, field string) (interface{}, error) {
	fullSecret, err := s.Get(mac, id)
	if err != nil {
		return nil, err
	}
	// TODO: type checking
	secretMap := fullSecret.(map[string]interface{})
	// TODO: not found checking
	return secretMap[field], nil
}

func (s *localSecrets) generate(mac string, secretDefs []SecretDef) error {
	hostSecrets, ok := s.hostToSecrets[mac]
	if !ok {
		s.hostToSecrets[mac] = make(map[string]interface{})
		hostSecrets = s.hostToSecrets[mac]
	}

	needsSave := false
	for _, def := range secretDefs {
		_, secretExists := hostSecrets[def.ID]
		if !secretExists {
			switch def.Type {
			case "password":
				// TODO: test for length
				var err error
				hostSecrets[def.ID], err = s.generatePassword(def.Opts)
				if err != nil {
					return err
				}
			case "ssh_key":
				var err error
				hostSecrets[def.ID], err = s.generateSSHKey(def.Opts)
				if err != nil {
					return err
				}
			default:
				// TODO: raise error if unknown type
			}
			needsSave = true
		}
	}

	if needsSave {
		return s.save()
	}
	return nil
}

func (s *localSecrets) save() error {
	updatedConfig := secretsConfig{
		Hosts: make([]hostSecrets, 0, len(s.hostToSecrets)),
	}
	for host, secrets := range s.hostToSecrets {
		updatedSecrets := hostSecrets{
			Mac:     host,
			Secrets: make([]secret, 0, len(secrets)),
		}
		for k, v := range secrets {
			updatedSecrets.Secrets = append(updatedSecrets.Secrets, secret{
				ID:    k,
				Value: v,
			})
		}
		updatedConfig.Hosts = append(updatedConfig.Hosts, updatedSecrets)
	}

	storeFile, err := os.OpenFile(s.storePath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	writer := yaml.NewEncoder(storeFile)
	err = writer.Encode(updatedConfig)
	if err != nil {
		return err
	}
	err = writer.Close()
	if err != nil {
		return err
	}
	err = storeFile.Close()
	if err != nil {
		return err
	}

	return nil
}

func (s *localSecrets) generatePassword(opts map[string]interface{}) (string, error) {
	length := 20
	rawLength, ok := opts["length"]
	if ok {
		length = rawLength.(int)
	}

	output := make([]rune, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(allChars))))
		if err != nil {
			return "", err
		}
		output[i] = rune(allChars[n.Int64()])
	}
	return string(output), nil
}

func (s *localSecrets) generateSSHKey(opts map[string]interface{}) (map[string]interface{}, error) {
	comment := ""
	rawComment, ok := opts["comment"]
	if ok {
		comment = rawComment.(string)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	var privateKeyContents bytes.Buffer
	if err := pem.Encode(&privateKeyContents, privateKeyPEM); err != nil {
		return nil, err
	}

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	publicKeyContents := string(ssh.MarshalAuthorizedKey(publicKey))

	if comment != "" {
		publicKeyContents = fmt.Sprintf("%s %s\n", strings.TrimSpace(publicKeyContents), comment)
	}

	return map[string]interface{}{
		"public_key":  string(publicKeyContents),
		"private_key": privateKeyContents.String(),
	}, nil
}
