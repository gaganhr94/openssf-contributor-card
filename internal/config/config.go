// Package config loads the project list and exclusion rules from data/.
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Maturity string

const (
	Graduated  Maturity = "graduated"
	Incubating Maturity = "incubating"
	Sandbox    Maturity = "sandbox"
)

type Project struct {
	Slug     string   `yaml:"slug"`
	Name     string   `yaml:"name"`
	Maturity Maturity `yaml:"maturity"`
	URL      string   `yaml:"url"`
	Repos    []string `yaml:"repos"`
}

type Projects struct {
	Projects []Project `yaml:"projects"`
}

func LoadProjects(path string) (*Projects, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var p Projects
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Projects) validate() error {
	seenSlug := map[string]bool{}
	seenRepo := map[string]string{}
	for _, proj := range p.Projects {
		if proj.Slug == "" {
			return fmt.Errorf("project %q: missing slug", proj.Name)
		}
		if seenSlug[proj.Slug] {
			return fmt.Errorf("duplicate project slug: %s", proj.Slug)
		}
		seenSlug[proj.Slug] = true
		switch proj.Maturity {
		case Graduated, Incubating, Sandbox:
		default:
			return fmt.Errorf("project %s: invalid maturity %q", proj.Slug, proj.Maturity)
		}
		if len(proj.Repos) == 0 {
			return fmt.Errorf("project %s: no repos", proj.Slug)
		}
		for _, r := range proj.Repos {
			if !strings.Contains(r, "/") {
				return fmt.Errorf("project %s: repo %q must be owner/name", proj.Slug, r)
			}
			if other, ok := seenRepo[strings.ToLower(r)]; ok {
				return fmt.Errorf("repo %s listed under both %s and %s", r, other, proj.Slug)
			}
			seenRepo[strings.ToLower(r)] = proj.Slug
		}
	}
	return nil
}

type Exclusions struct {
	Excluded []string `yaml:"excluded"`
}

type ExclusionMatcher struct {
	literals map[string]bool
	patterns []*regexp.Regexp
}

var botRegexp = regexp.MustCompile(`(?i).+\[bot\]$`)

func LoadExclusions(path string) (*ExclusionMatcher, error) {
	m := &ExclusionMatcher{
		literals: map[string]bool{},
		patterns: []*regexp.Regexp{botRegexp},
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var e Exclusions
	if err := yaml.Unmarshal(b, &e); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, login := range e.Excluded {
		m.literals[strings.ToLower(login)] = true
	}
	return m, nil
}

func (m *ExclusionMatcher) IsExcluded(login string) bool {
	if m.literals[strings.ToLower(login)] {
		return true
	}
	for _, p := range m.patterns {
		if p.MatchString(login) {
			return true
		}
	}
	return false
}
