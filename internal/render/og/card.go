// Package og generates per-contributor Open Graph images at build time so
// shared links produce rich, personalized previews on social platforms.
//
// The card design intentionally diverges from the HTML card on the site —
// social-media scrapers don't run JS, can't load CSS, and crop aggressively,
// so the image is purpose-built: 1200×630, large name, three big stat tiles,
// avatar, and a short project list.
package og

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"

	"github.com/ghr/openssf-contributor-card/internal/avatar"
	"github.com/ghr/openssf-contributor-card/internal/store"
)

const (
	cardWidth  = 1200
	cardHeight = 630
	avatarSize = 220
	jpegQuality = 85
)

// Theme colors — must stay in sync with the dark theme in web/static/css/style.css.
var (
	colorBG       = mustParseHex("#0b1020")
	colorCard     = mustParseHex("#131a2c")
	colorAccent   = mustParseHex("#4f8cff")
	colorAccentBG = mustParseHex("#1c2a4a")
	colorFG       = mustParseHex("#e9eef7")
	colorMuted    = mustParseHex("#97a3b6")
	colorBorder   = mustParseHex("#1f2945")
)

// Renderer generates JPEG OG cards into outDir/og/<login>.jpg.
type Renderer struct {
	outDir   string
	avatars  *avatar.Cache
	siteName string

	regular font.Face
	bold    font.Face
	xlBold  font.Face
	stat    font.Face
	mono    font.Face // unused for now, reserved for stat numerals
	once    sync.Once
	initErr error
}

func New(outDir string, avatars *avatar.Cache) *Renderer {
	return &Renderer{
		outDir:   outDir,
		avatars:  avatars,
		siteName: "OpenSSF Contributor Card",
	}
}

// Render writes the OG card for one contributor.
func (r *Renderer) Render(ctx context.Context, c store.ContributorAggregate) error {
	if err := r.init(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(r.outDir, "og"), 0o755); err != nil {
		return err
	}

	dc := gg.NewContext(cardWidth, cardHeight)

	// Background
	dc.SetColor(colorBG)
	dc.Clear()

	// Header strip
	dc.SetColor(colorCard)
	dc.DrawRectangle(0, 0, cardWidth, 80)
	dc.Fill()
	dc.SetColor(colorBorder)
	dc.DrawRectangle(0, 79, cardWidth, 1)
	dc.Fill()

	dc.SetColor(colorFG)
	dc.SetFontFace(r.bold)
	dc.DrawStringAnchored("OpenSSF", 50, 40, 0, 0.5)
	openssfW, _ := dc.MeasureString("OpenSSF")
	dc.SetFontFace(r.regular)
	dc.SetColor(colorMuted)
	dc.DrawStringAnchored("Contributor Card", 50+openssfW+18, 40, 0, 0.5)

	dc.SetColor(colorMuted)
	dc.SetFontFace(r.regular)
	dc.DrawStringAnchored(fmt.Sprintf("@%s", c.Login), float64(cardWidth-50), 40, 1, 0.5)

	// Avatar
	avatarX := 80.0
	avatarY := 130.0
	if path, err := r.avatarPath(ctx, c); err == nil && path != "" {
		if err := drawCircularImage(dc, path, avatarX, avatarY, avatarSize); err == nil {
			// success
		} else {
			drawAvatarFallback(dc, avatarX, avatarY, avatarSize, c.Login)
		}
	} else {
		drawAvatarFallback(dc, avatarX, avatarY, avatarSize, c.Login)
	}

	// Display name + login
	textX := avatarX + avatarSize + 40
	dc.SetColor(colorFG)
	dc.SetFontFace(r.xlBold)
	name := c.DisplayName
	if name == "" {
		name = c.Login
	}
	dc.DrawStringAnchored(truncate(name, 28), textX, avatarY+60, 0, 0.5)

	// Hero subtitle: "<contributions> contributions to <repo_count> repositories".
	// Matches the website's prominent stat line. Drawn on two lines using the
	// regular font so the numbers stand out via the bold xlBold above.
	dc.SetColor(colorMuted)
	dc.SetFontFace(r.regular)
	subtitle := fmt.Sprintf("%d contributions to %d repositor%s",
		c.TotalContributions, c.RepoCount, pluralizeRepo(c.RepoCount))
	dc.DrawStringAnchored(subtitle, textX, avatarY+120, 0, 0.5)
	if c.SinceYear() > 0 {
		var since string
		if y := c.YearsActive(); y > 0 {
			since = fmt.Sprintf("Contributing since %d  ·  %d year%s", c.SinceYear(), y, pluralWord("", y))
		} else {
			since = fmt.Sprintf("Contributing since %d", c.SinceYear())
		}
		dc.DrawStringAnchored(since, textX, avatarY+160, 0, 0.5)
	}

	// Stat tiles: 4 across (commits / PRs / issues / total contributions).
	tileY := avatarY + avatarSize + 40
	tileH := 130.0
	tileGap := 18.0
	tileW := (cardWidth - 80*2 - tileGap*3) / 4
	r.drawStatTile(dc, 80, tileY, tileW, tileH,
		fmt.Sprintf("%d", c.TotalCommits), "commits", false)
	r.drawStatTile(dc, 80+(tileW+tileGap), tileY, tileW, tileH,
		fmt.Sprintf("%d", c.TotalPRs), "PRs", false)
	r.drawStatTile(dc, 80+(tileW+tileGap)*2, tileY, tileW, tileH,
		fmt.Sprintf("%d", c.TotalIssues), "issues", false)
	r.drawStatTile(dc, 80+(tileW+tileGap)*3, tileY, tileW, tileH,
		fmt.Sprintf("%d", c.TotalContributions), "contributions", true)

	// Project pills
	pillY := tileY + tileH + 30
	r.drawProjectPills(dc, 80, pillY, c.Projects)

	dst := filepath.Join(r.outDir, "og", c.Login+".jpg")
	return saveJPEG(dc.Image(), dst, jpegQuality)
}

// RenderAll renders one OG card per contributor and the site-wide default
// card used by the homepage. Returns on the first error; the avatar cache is
// shared so retries reuse downloaded files.
func (r *Renderer) RenderAll(ctx context.Context, cs []store.ContributorAggregate, projectCount int) error {
	if err := r.RenderDefault(ctx, len(cs), projectCount); err != nil {
		return fmt.Errorf("render default og: %w", err)
	}
	for i, c := range cs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := r.Render(ctx, c); err != nil {
			return fmt.Errorf("render og card for %s (%d/%d): %w", c.Login, i+1, len(cs), err)
		}
	}
	return nil
}

// RenderDefault writes the homepage OG image with summary numbers.
// Output: <outDir>/og/index.jpg
func (r *Renderer) RenderDefault(ctx context.Context, contributorCount, projectCount int) error {
	if err := r.init(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(r.outDir, "og"), 0o755); err != nil {
		return err
	}

	dc := gg.NewContext(cardWidth, cardHeight)
	dc.SetColor(colorBG)
	dc.Clear()

	// Tall accent bar at the top
	dc.SetColor(colorCard)
	dc.DrawRectangle(0, 0, cardWidth, 80)
	dc.Fill()
	dc.SetColor(colorBorder)
	dc.DrawRectangle(0, 79, cardWidth, 1)
	dc.Fill()

	dc.SetColor(colorFG)
	dc.SetFontFace(r.bold)
	dc.DrawStringAnchored("OpenSSF", 50, 40, 0, 0.5)
	openssfW, _ := dc.MeasureString("OpenSSF")
	dc.SetFontFace(r.regular)
	dc.SetColor(colorMuted)
	dc.DrawStringAnchored("Contributor Card", 50+openssfW+18, 40, 0, 0.5)

	// Big headline
	dc.SetColor(colorFG)
	dc.SetFontFace(r.xlBold)
	dc.DrawStringAnchored("Celebrating contributors", cardWidth/2, 220, 0.5, 0.5)
	dc.DrawStringAnchored("to OpenSSF projects", cardWidth/2, 290, 0.5, 0.5)

	// Stat strip
	dc.SetColor(colorMuted)
	dc.SetFontFace(r.regular)
	subtitle := fmt.Sprintf("%d contributors  ·  %d projects",
		contributorCount, projectCount)
	dc.DrawStringAnchored(subtitle, cardWidth/2, 380, 0.5, 0.5)

	dc.SetColor(colorAccent)
	dc.SetFontFace(r.bold)
	dc.DrawStringAnchored("Find yours, share your card", cardWidth/2, 460, 0.5, 0.5)

	dst := filepath.Join(r.outDir, "og", "index.jpg")
	return saveJPEG(dc.Image(), dst, jpegQuality)
}

func (r *Renderer) init() error {
	r.once.Do(func() {
		reg, err := opentype.Parse(goregular.TTF)
		if err != nil {
			r.initErr = err
			return
		}
		bold, err := opentype.Parse(gobold.TTF)
		if err != nil {
			r.initErr = err
			return
		}
		r.regular = mustNewFace(reg, 28)
		r.bold = mustNewFace(bold, 32)
		r.xlBold = mustNewFace(bold, 56)
		r.stat = mustNewFace(bold, 72)
	})
	return r.initErr
}

func (r *Renderer) avatarPath(ctx context.Context, c store.ContributorAggregate) (string, error) {
	if !avatar.IsHTTPURL(c.AvatarURL) {
		return "", nil
	}
	return r.avatars.Path(ctx, c.Login, c.AvatarURL)
}

func (r *Renderer) drawStatTile(dc *gg.Context, x, y, w, h float64, value, label string, highlight bool) {
	if highlight {
		dc.SetColor(colorAccent)
	} else {
		dc.SetColor(colorAccentBG)
	}
	dc.DrawRoundedRectangle(x, y, w, h, 14)
	dc.Fill()

	if highlight {
		dc.SetColor(color.White)
	} else {
		dc.SetColor(colorFG)
	}
	dc.SetFontFace(r.stat)
	dc.DrawStringAnchored(value, x+w/2, y+h/2-12, 0.5, 0.5)

	if highlight {
		dc.SetColor(color.RGBA{0xff, 0xff, 0xff, 0xb4}) // ~70% white
	} else {
		dc.SetColor(colorMuted)
	}
	dc.SetFontFace(r.regular)
	dc.DrawStringAnchored(strings.ToUpper(label), x+w/2, y+h-30, 0.5, 0.5)
}

func (r *Renderer) drawProjectPills(dc *gg.Context, x, y float64, projects []store.ProjectSummary) {
	if len(projects) == 0 {
		return
	}
	sorted := append([]store.ProjectSummary(nil), projects...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	const pillH = 56.0
	const pillGap = 12.0
	const pillPad = 28.0
	maxX := float64(cardWidth - 80)

	dc.SetFontFace(r.regular)
	cur := x
	shown := 0
	for i, p := range sorted {
		w, _ := dc.MeasureString(p.Name)
		pillW := w + pillPad*2
		if cur+pillW > maxX {
			more := len(sorted) - i
			if more > 0 {
				label := fmt.Sprintf("+%d more", more)
				mw, _ := dc.MeasureString(label)
				dc.SetColor(colorAccentBG)
				dc.DrawRoundedRectangle(cur, y, mw+pillPad*2, pillH, pillH/2)
				dc.Fill()
				dc.SetColor(colorMuted)
				dc.DrawStringAnchored(label, cur+(mw+pillPad*2)/2, y+pillH/2, 0.5, 0.5)
			}
			break
		}
		dc.SetColor(colorAccentBG)
		dc.DrawRoundedRectangle(cur, y, pillW, pillH, pillH/2)
		dc.Fill()
		dc.SetColor(colorFG)
		dc.DrawStringAnchored(p.Name, cur+pillW/2, y+pillH/2, 0.5, 0.5)
		cur += pillW + pillGap
		shown++
	}
	_ = shown
}

func drawCircularImage(dc *gg.Context, path string, x, y, size float64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	// Scale via gg.NewContextForImage and fill a clipped circle.
	dc.Push()
	defer dc.Pop()
	dc.DrawCircle(x+size/2, y+size/2, size/2)
	dc.Clip()
	scaled := scaleImage(img, int(size))
	dc.DrawImage(scaled, int(x), int(y))
	dc.ResetClip()

	// Border
	dc.SetColor(colorBorder)
	dc.SetLineWidth(4)
	dc.DrawCircle(x+size/2, y+size/2, size/2)
	dc.Stroke()
	return nil
}

func drawAvatarFallback(dc *gg.Context, x, y, size float64, login string) {
	dc.SetColor(colorAccentBG)
	dc.DrawCircle(x+size/2, y+size/2, size/2)
	dc.Fill()

	dc.SetColor(colorFG)
	face, err := opentype.Parse(gobold.TTF)
	if err == nil {
		f := mustNewFace(face, 90)
		dc.SetFontFace(f)
	}
	initial := strings.ToUpper(string([]rune(login + "?")[0]))
	dc.DrawStringAnchored(initial, x+size/2, y+size/2, 0.5, 0.5)
}

// scaleImage scales src to fit a square of dim×dim using the simplest path:
// gg's NewContextForImage + Resize. Lanczos would be better but for 220px
// avatars the difference is invisible after JPEG compression.
func scaleImage(src image.Image, dim int) image.Image {
	dc := gg.NewContext(dim, dim)
	bounds := src.Bounds()
	srcW := float64(bounds.Dx())
	srcH := float64(bounds.Dy())
	scale := float64(dim) / minF(srcW, srcH)
	dc.Scale(scale, scale)
	offX := (float64(dim)/scale - srcW) / 2
	offY := (float64(dim)/scale - srcH) / 2
	dc.DrawImage(src, int(offX), int(offY))
	return dc.Image()
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func mustNewFace(f *opentype.Font, points float64) font.Face {
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size: points,
		DPI:  72,
	})
	if err != nil {
		panic(err)
	}
	return face
}

func mustParseHex(s string) color.Color {
	if s[0] == '#' {
		s = s[1:]
	}
	if len(s) != 6 {
		panic("invalid hex color: " + s)
	}
	parse := func(b byte) uint8 {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		}
		panic("bad hex digit")
	}
	return color.RGBA{
		R: parse(s[0])<<4 | parse(s[1]),
		G: parse(s[2])<<4 | parse(s[3]),
		B: parse(s[4])<<4 | parse(s[5]),
		A: 0xff,
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func pluralWord(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// pluralizeRepo returns the suffix completing "repositor[y|ies]".
func pluralizeRepo(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
