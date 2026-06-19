package yamlconfig

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/lxcfh/lxcfh/internal/models"
	"golang.org/x/crypto/ssh"
)

func ToConfigDocument(doc models.ConfigYAMLDocument) (models.ConfigDocument, error) {
	credByName := make(map[string]string, len(doc.Credentials))
	creds := make([]models.Credential, 0, len(doc.Credentials))
	for _, c := range doc.Credentials {
		id := uuid.NewString()
		credByName[c.Name] = id
		creds = append(creds, models.Credential{
			ID:       id,
			Name:     c.Name,
			Type:     c.Type,
			Username: c.Username,
		})
	}

	keyByName := make(map[string]string, len(doc.Keys))
	keys := make([]models.SSHKey, 0, len(doc.Keys))
	for _, k := range doc.Keys {
		fp, err := fingerprintPublicKey(k.PublicKey)
		if err != nil {
			return models.ConfigDocument{}, fmt.Errorf("key %s: %w", k.Name, err)
		}
		id := uuid.NewString()
		keyByName[k.Name] = id
		keys = append(keys, models.SSHKey{
			ID:          id,
			Name:        k.Name,
			Fingerprint: fp,
			PublicKey:   k.PublicKey,
			Comment:     k.Comment,
		})
	}

	defaultPort := 22
	if doc.Settings != nil && doc.Settings.DefaultNodePort > 0 {
		defaultPort = doc.Settings.DefaultNodePort
	}

	nodes := make([]models.Node, 0, len(doc.Nodes))
	for _, spec := range doc.Nodes {
		enabled := true
		if spec.Enabled != nil {
			enabled = *spec.Enabled
		}
		port := spec.Port
		if port == 0 {
			port = defaultPort
		}
		node := models.Node{
			ID:            uuid.NewString(),
			Name:          spec.Name,
			Host:          spec.Host,
			Port:          port,
			Username:      spec.Username,
			Labels:        spec.Labels,
			Enabled:       enabled,
			HostKeyStatus: "unknown",
		}
		if spec.Credential != "" {
			id, ok := credByName[spec.Credential]
			if !ok {
				return models.ConfigDocument{}, fmt.Errorf("credential not found: %s", spec.Credential)
			}
			node.CredentialID = id
		}
		if spec.Key != "" {
			id, ok := keyByName[spec.Key]
			if !ok {
				return models.ConfigDocument{}, fmt.Errorf("key not found: %s", spec.Key)
			}
			node.KeyID = id
		}
		nodes = append(nodes, node)
	}

	users := make([]models.User, 0, len(doc.Users))
	for _, u := range doc.Users {
		enabled := true
		if u.Enabled != nil {
			enabled = *u.Enabled
		}
		users = append(users, models.User{
			ID:          uuid.NewString(),
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Email:       u.Email,
			Role:        u.Role,
			Enabled:     enabled,
		})
	}

	version := doc.Version
	if version <= 0 {
		version = 1
	}

	settings := models.Settings{}
	if doc.Settings != nil {
		settings = *doc.Settings
	}

	return models.ConfigDocument{
		Version:     version,
		Settings:    settings,
		Nodes:       nodes,
		Credentials: creds,
		Keys:        keys,
		Users:       users,
	}, nil
}

func fingerprintPublicKey(publicKey string) (string, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return "", fmt.Errorf("invalid public key: %w", err)
	}
	return ssh.FingerprintSHA256(pub), nil
}
