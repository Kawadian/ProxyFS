//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type setupResponse struct {
	User struct {
		Username string `json:"username"`
	} `json:"user"`
	CSRFToken string `json:"csrf_token"`
}

func startHub(t *testing.T) (baseURL string, sftpPort int, cleanup func()) {
	t.Helper()

	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "lxcfh.db")
	masterKeyPath := filepath.Join(dataDir, "master.key")
	if err := os.WriteFile(masterKeyPath, []byte("integration-test-master-key-32b!"), 0o600); err != nil {
		t.Fatalf("master key: %v", err)
	}

	httpPort := freePort(t)
	sftpPort = freePort(t)

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	bin := filepath.Join(repoRoot, "bin", "lxcfh")
	if _, err := os.Stat(bin); err != nil {
		build := exec.Command("go", "build", "-o", bin, "./cmd/lxcfh")
		build.Dir = repoRoot
		build.Env = append(os.Environ(), "CGO_ENABLED=1")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build hub: %v\n%s", err, out)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = append(os.Environ(),
		"LXCFH_DATA_DIR="+dataDir,
		"LXCFH_DB_PATH="+dbPath,
		"LXCFH_BIND_HOST=127.0.0.1",
		fmt.Sprintf("LXCFH_BIND_PORT=%d", httpPort),
		fmt.Sprintf("LXCFH_SFTP_PORT=%d", sftpPort),
		"LXCFH_MASTER_KEY_PATH="+masterKeyPath,
		"LXCFH_FUSE_MOUNT="+filepath.Join(dataDir, "fuse"),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start hub: %v", err)
	}

	baseURL = fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	waitForHTTP(t, baseURL+"/health/live", 30*time.Second)

	cleanup = func() {
		cancel()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	return baseURL, sftpPort, cleanup
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForHTTP(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		res, err := client.Get(url)
		if err == nil {
			_, _ = io.Copy(io.Discard, res.Body)
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", url)
}

func setupAdmin(t *testing.T, baseURL string) (username, password, csrf, cookie string) {
	t.Helper()
	username = "admin"
	password = "integration-pass-123"
	body, _ := json.Marshal(map[string]string{
		"username":     username,
		"password":     password,
		"display_name": "Admin",
	})
	res, err := http.Post(baseURL+"/api/v1/auth/setup", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("setup status %d: %s", res.StatusCode, b)
	}
	var parsed setupResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode setup: %v", err)
	}
	for _, c := range res.Cookies() {
		if c.Name == "lxcfh_session" {
			cookie = c.Value
		}
	}
	return username, password, parsed.CSRFToken, cookie
}

func TestHubProtocolsConnectivity(t *testing.T) {
	baseURL, sftpPort, cleanup := startHub(t)
	defer cleanup()

	username, password, csrf, cookie := setupAdmin(t, baseURL)

	t.Run("SFTP", func(t *testing.T) {
		config := &ssh.ClientConfig{
			User:            username,
			Auth:            []ssh.AuthMethod{ssh.Password(password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         10 * time.Second,
		}
		conn, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", sftpPort), config)
		if err != nil {
			t.Fatalf("ssh dial: %v", err)
		}
		defer conn.Close()

		client, err := sftp.NewClient(conn)
		if err != nil {
			t.Fatalf("sftp client: %v", err)
		}
		defer client.Close()

		if _, err := client.ReadDir("/"); err != nil {
			t.Fatalf("sftp readdir: %v", err)
		}
	})

	t.Run("WebDAVRoot", func(t *testing.T) {
		req, err := http.NewRequest("PROPFIND", baseURL+"/", nil)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		req.SetBasicAuth(username, password)
		req.Header.Set("Depth", "1")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("propfind: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusMultiStatus {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("status %d: %s", res.StatusCode, b)
		}
	})

	t.Run("WebDAVLegacyPath", func(t *testing.T) {
		req, err := http.NewRequest("PROPFIND", baseURL+"/dav/", nil)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		req.SetBasicAuth(username, password)
		req.Header.Set("Depth", "0")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("propfind: %v", err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusMultiStatus {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("status %d: %s", res.StatusCode, b)
		}
	})

	t.Run("SMB", func(t *testing.T) {
		if _, err := exec.LookPath("smbd"); err != nil {
			t.Skip("smbd not installed")
		}
		if _, err := os.Stat("/dev/fuse"); err != nil {
			t.Skip("FUSE unavailable")
		}

		req, err := http.NewRequest(http.MethodPatch, baseURL+"/api/v1/protocols/smb", strings.NewReader(`{"enabled":true}`))
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		req.AddCookie(&http.Cookie{Name: "lxcfh_session", Value: cookie})

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("enable smb: %v", err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("enable smb status %d: %s", res.StatusCode, body)
		}

		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			var overview struct {
				Protocols []struct {
					Name    string `json:"name"`
					Running bool   `json:"running"`
					Message string `json:"message"`
				} `json:"protocols"`
			}
			statusReq, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/protocols", nil)
			statusReq.AddCookie(&http.Cookie{Name: "lxcfh_session", Value: cookie})
			statusRes, err := http.DefaultClient.Do(statusReq)
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			_ = json.NewDecoder(statusRes.Body).Decode(&overview)
			statusRes.Body.Close()
			for _, p := range overview.Protocols {
				if p.Name == "smb" && p.Running {
					goto smbReady
				}
				if p.Name == "smb" && p.Message != "" {
					t.Fatalf("smb failed: %s", p.Message)
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		t.Fatal("smb did not start")

	smbReady:
		if _, err := exec.LookPath("smbclient"); err != nil {
			t.Skip("smbclient not installed")
		}
		share := os.Getenv("SMB_SHARE_NAME")
		if share == "" {
			share = "lxcfh"
		}
		cmd := exec.Command("smbclient", "//127.0.0.1/"+share, "-U", username+"%"+password, "-c", "ls")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("smbclient: %v\n%s", err, out)
		}
	})
}
