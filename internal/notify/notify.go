package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rtf6x/mogotor/internal/models"
)

type Config struct {
	URL       string
	StatePath string
	Hostname  string
}

type Notifier struct {
	url       string
	statePath string
	hostname  string
	Location  *time.Location
	client    *http.Client
}

func New(cfg Config) *Notifier {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		loc = time.UTC
	}
	hostname := strings.TrimSpace(cfg.Hostname)
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	return &Notifier{
		url:       strings.TrimSpace(cfg.URL),
		statePath: cfg.StatePath,
		hostname:  hostname,
		Location:  loc,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *Notifier) MaybeSend(snap models.Snapshot, now time.Time) error {
	if n == nil || n.url == "" {
		return nil
	}
	last, _ := n.lastSent()
	if !ShouldSendDaily(now, last, n.Location) {
		return nil
	}
	if err := n.post(FormatDigest(n.hostname, snap)); err != nil {
		return err
	}
	return n.markSent(now)
}

func (n *Notifier) post(text string) error {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notify: %s", resp.Status)
	}
	return nil
}

func (n *Notifier) lastSent() (time.Time, error) {
	raw, err := os.ReadFile(n.statePath)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(string(raw)))
}

func (n *Notifier) markSent(now time.Time) error {
	if n.statePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(n.statePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(n.statePath, []byte(now.Format(time.RFC3339)+"\n"), 0o644)
}
