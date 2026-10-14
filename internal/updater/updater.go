package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const apiBase = "https://api.github.com/repos"

type Updater struct {
	owner string
	repo  string
	client *http.Client
}

type UpdateInfo struct {
	HasUpdate bool   `json:"hasUpdate"`
	Version   string `json:"version"`
	URL       string `json:"url"`
	Body      string `json:"body"`
	Published string `json:"published"`
}

func New(owner, repo string) *Updater {
	return &Updater{
		owner: owner, repo: repo,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (u *Updater) Check(current string) (*UpdateInfo, error) {
	url := fmt.Sprintf("%s/%s/%s/releases/latest", apiBase, u.owner, u.repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "TSGMemOptimizer/1.0")
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github api %d", resp.StatusCode)
	}
	var data struct {
		TagName     string `json:"tag_name"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Assets      []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	latest := strings.TrimPrefix(data.TagName, "v")
	info := &UpdateInfo{
		Version:   latest,
		Body:      data.Body,
		Published: data.PublishedAt,
		HasUpdate: compareVersion(latest, current) > 0,
	}
	for _, a := range data.Assets {
		if strings.Contains(a.Name, runtime.GOOS) && strings.Contains(a.Name, runtime.GOARCH) {
			info.URL = a.URL
			break
		}
	}
	return info, nil
}

func (u *Updater) Download(url, version string) error {
	if url == "" {
		return fmt.Errorf("no download url")
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "TSGMemOptimizer/1.0")
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download %d", resp.StatusCode)
	}
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	tmp := filepath.Join(dir, fmt.Sprintf(".tsgmemo-update-%s", version))
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		return err
	}
	_ = os.Chmod(tmp, 0755)
	// rename on next restart via wrapper script / manual
	marker := filepath.Join(dir, ".tsgmemo-update-marker")
	_ = os.WriteFile(marker, []byte(tmp+"\n"+exe), 0644)
	return nil
}

func compareVersion(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, _ := parseInt(pa[i])
		nb, _ := parseInt(pb[i])
		if na > nb {
			return 1
		}
		if na < nb {
			return -1
		}
	}
	if len(pa) > len(pb) {
		return 1
	}
	if len(pa) < len(pb) {
		return -1
	}
	return 0
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
