// Package avatar downloads and caches GitHub avatars for use in OG card
// rendering. Avatars are downloaded once per contributor and re-used across
// builds. Each cached file is the raw response body from GitHub; we don't
// re-encode at fetch time so cache files are small (typically 30-100KB).
package avatar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Cache struct {
	dir    string
	http   *http.Client
	once   sync.Once
	mkErr  error
}

func NewCache(dir string) *Cache {
	return &Cache{
		dir:  dir,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Cache) ensureDir() error {
	c.once.Do(func() {
		c.mkErr = os.MkdirAll(c.dir, 0o755)
	})
	return c.mkErr
}

// Path returns the on-disk cache path for the given login. If the file
// already exists, no fetch happens. Otherwise the avatar is downloaded
// from avatarURL and stored.
//
// Returns the empty string and a nil error if avatarURL is empty (caller
// should fall back to a placeholder).
func (c *Cache) Path(ctx context.Context, login, avatarURL string) (string, error) {
	if avatarURL == "" {
		return "", nil
	}
	if err := c.ensureDir(); err != nil {
		return "", err
	}
	// Cache files are keyed on (login, avatarURL hash) so a changed avatar
	// URL invalidates the cache automatically.
	hash := sha256.Sum256([]byte(avatarURL))
	name := fmt.Sprintf("%s-%s", sanitize(login), hex.EncodeToString(hash[:6]))
	dst := filepath.Join(c.dir, name)
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}
	return dst, c.fetch(ctx, avatarURL, dst)
}

func (c *Cache) fetch(ctx context.Context, src, dst string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", src, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "openssf-contribcard/0.1")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("fetch avatar: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("avatar %s: http %d", src, resp.StatusCode)
	}
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// sanitize maps a login to a filesystem-safe name. GitHub logins are
// alphanumeric + dash + underscore so this is mostly a defensive measure.
func sanitize(login string) string {
	out := make([]byte, 0, len(login))
	for i := 0; i < len(login); i++ {
		c := login[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// IsHTTPURL is a convenience to validate avatar URLs before attempting to fetch.
func IsHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}
