// Package html renders the static site from the SQLite store.
//
// Each contributor gets a real HTML page with personalized OG metadata so
// shared links produce rich previews on Twitter/LinkedIn/Slack — the reason
// we diverged from CNCF's SPA-with-shared-OG architecture.
package html

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ghr/openssf-contributor-card/internal/store"
)

type Options struct {
	SiteURL  string // absolute URL with no trailing slash, e.g. https://owner.github.io/repo
	BasePath string // URL path prefix derived from SiteURL: "/repo" for project Pages, "" for root deploys
	OutDir   string // dist/
	TopN     int    // how many tiles to render server-side on the index
	GenAt    time.Time
}

// DefaultOptions parses the site URL to derive BasePath. GitHub Pages on a
// project repo serves under /<repo>/, so internal links and assets must be
// prefixed. For user pages or custom domains at root, BasePath is empty.
func DefaultOptions(siteURL, outDir string) Options {
	siteURL = strings.TrimRight(siteURL, "/")
	basePath := ""
	if u, err := url.Parse(siteURL); err == nil {
		basePath = strings.TrimRight(u.Path, "/")
	}
	return Options{
		SiteURL:  siteURL,
		BasePath: basePath,
		OutDir:   outDir,
		TopN:     120,
		GenAt:    time.Now().UTC(),
	}
}

type Renderer struct {
	opt        Options
	tmpl       *template.Template
	templateFS fs.FS
	staticFS   fs.FS
}

// New constructs a Renderer that will read templates and static assets from
// the given filesystems. Pass the //go:embed FS for production or os.DirFS
// for local development.
func New(opt Options, templates, static fs.FS) (*Renderer, error) {
	r := &Renderer{opt: opt, templateFS: templates, staticFS: static}
	if err := r.parseTemplates(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Renderer) parseTemplates() error {
	t := template.New("").Funcs(template.FuncMap{})
	// Parse layout once; per-page templates are parsed alongside in Render.
	layout, err := fs.ReadFile(r.templateFS, "_layout.tmpl")
	if err != nil {
		return fmt.Errorf("read layout: %w", err)
	}
	if _, err := t.Parse(string(layout)); err != nil {
		return fmt.Errorf("parse layout: %w", err)
	}
	r.tmpl = t
	return nil
}

// Render emits the full site to opt.OutDir.
func (r *Renderer) Render(ctx context.Context, st *store.Store) error {
	if err := os.MkdirAll(r.opt.OutDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(r.opt.OutDir, "c"), 0o755); err != nil {
		return err
	}

	// Copy static assets first.
	if err := r.copyStatic(); err != nil {
		return fmt.Errorf("copy static: %w", err)
	}

	contributors, err := st.AllContributorAggregates(ctx)
	if err != nil {
		return fmt.Errorf("query contributors: %w", err)
	}
	projects, err := st.AllProjects(ctx)
	if err != nil {
		return fmt.Errorf("query projects: %w", err)
	}
	slog.Info("rendering site",
		"contributors", len(contributors),
		"projects", len(projects),
		"out", r.opt.OutDir)

	if err := r.renderContributors(contributors); err != nil {
		return fmt.Errorf("contributor pages: %w", err)
	}
	if err := r.renderIndex(contributors, projects); err != nil {
		return fmt.Errorf("index page: %w", err)
	}
	return nil
}

type contributorView struct {
	store.ContributorAggregate
	SiteURL   string
	BasePath  string
	ShareText string
}

func (r *Renderer) renderContributors(cs []store.ContributorAggregate) error {
	pageTmpl, err := fs.ReadFile(r.templateFS, "contributor.tmpl")
	if err != nil {
		return err
	}
	t, err := r.tmpl.Clone()
	if err != nil {
		return err
	}
	if _, err := t.Parse(string(pageTmpl)); err != nil {
		return err
	}

	for _, c := range cs {
		out := filepath.Join(r.opt.OutDir, "c", c.Login+".html")
		f, err := os.Create(out)
		if err != nil {
			return err
		}
		view := contributorView{
			ContributorAggregate: c,
			SiteURL:              r.opt.SiteURL,
			BasePath:             r.opt.BasePath,
			ShareText:            shareText(c),
		}
		if err := t.ExecuteTemplate(f, "layout", view); err != nil {
			f.Close()
			return fmt.Errorf("execute contributor.tmpl for %s: %w", c.Login, err)
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	slog.Info("rendered contributor pages", "count", len(cs))
	return nil
}

// shareText returns plain text. html/template applies URL escaping in the
// href context, so we don't pre-escape here (doing so caused double encoding).
func shareText(c store.ContributorAggregate) string {
	return fmt.Sprintf("My OpenSSF Contributor Card: %d commits across %d project%s.",
		c.TotalCommits, len(c.Projects), plural(len(c.Projects)))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

type indexView struct {
	SiteURL    string
	BasePath   string
	Total      int
	Projects   []store.ProjectSummary
	Top        []indexTile
	SearchJSON template.JS // raw JSON; safe because we marshal it ourselves
}

type indexTile struct {
	Login        string
	DisplayName  string
	Commits      int
	ProjectSlugs []string
}

// searchEntry uses single-letter keys to keep the embedded JSON small.
type searchEntry struct {
	L string   `json:"l"`            // login
	N string   `json:"n,omitempty"`  // display name
	C int      `json:"c"`            // total commits
	P []string `json:"p,omitempty"`  // project slugs
}

func (r *Renderer) renderIndex(cs []store.ContributorAggregate, projects []store.ProjectSummary) error {
	pageTmpl, err := fs.ReadFile(r.templateFS, "index.tmpl")
	if err != nil {
		return err
	}
	t, err := r.tmpl.Clone()
	if err != nil {
		return err
	}
	if _, err := t.Parse(string(pageTmpl)); err != nil {
		return err
	}

	top := cs
	if r.opt.TopN > 0 && len(top) > r.opt.TopN {
		top = top[:r.opt.TopN]
	}
	tiles := make([]indexTile, len(top))
	for i, c := range top {
		slugs := projectSlugs(c.Projects)
		tiles[i] = indexTile{
			Login:        c.Login,
			DisplayName:  c.DisplayName,
			Commits:      c.TotalCommits,
			ProjectSlugs: slugs,
		}
	}

	entries := make([]searchEntry, len(cs))
	for i, c := range cs {
		entries[i] = searchEntry{
			L: c.Login,
			N: c.DisplayName,
			C: c.TotalCommits,
			P: projectSlugs(c.Projects),
		}
	}
	jsonB, err := json.Marshal(entries)
	if err != nil {
		return err
	}

	out := filepath.Join(r.opt.OutDir, "index.html")
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	view := indexView{
		SiteURL:    r.opt.SiteURL,
		BasePath:   r.opt.BasePath,
		Total:      len(cs),
		Projects:   projects,
		Top:        tiles,
		SearchJSON: template.JS(jsonB),
	}
	if err := t.ExecuteTemplate(f, "layout", view); err != nil {
		return fmt.Errorf("execute index.tmpl: %w", err)
	}
	slog.Info("rendered index", "tiles", len(tiles), "search_entries", len(entries))
	return nil
}

func projectSlugs(ps []store.ProjectSummary) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Slug
	}
	sort.Strings(out)
	return out
}

func (r *Renderer) copyStatic() error {
	return fs.WalkDir(r.staticFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." || d.IsDir() {
			if !d.IsDir() {
				return nil
			}
			return os.MkdirAll(filepath.Join(r.opt.OutDir, "static", path), 0o755)
		}
		dst := filepath.Join(r.opt.OutDir, "static", path)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		src, err := r.staticFS.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer out.Close()
		if _, err := io.Copy(out, src); err != nil {
			return err
		}
		return nil
	})
}
