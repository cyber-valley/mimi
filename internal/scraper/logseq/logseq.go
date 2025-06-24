package logseq

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	propertyRegexp = regexp.MustCompile(`(\w[\w_-]*\w):: (.+)$`)
	refRegexp      = regexp.MustCompile(`\[\[(@?[\w_-]*\w)\]\]`)
)

const (
	PageLevel  = "page"
	BlockLevel = "block"
)

type RegexGraph struct {
	path string
}

func NewRegexGraph(path string) RegexGraph {
	return RegexGraph{
		path: path,
	}
}

func (g RegexGraph) WalkPages() iter.Seq[Page] {
	return func(yield func(p Page) bool) {
		_ = filepath.Walk(g.path, func(path string, fs fs.FileInfo, err error) error {
			if err != nil {
				slog.Error("failed to access path", "path", path, "with", err)
				return nil
			}

			if !strings.HasSuffix(fs.Name(), ".md") {
				return nil
			}

			page, err := NewPage(path)
			if err != nil {
				slog.Error("failed to create new page", "with", err)
				return nil
			}

			if !yield(page) {
				return fmt.Errorf("pages walk iteration stopped")
			}

			return nil
		})
	}
}

type Page struct {
	Path string
	Info PageInfo
}

func NewPage(path string) (Page, error) {
	// Open page for reading
	file, err := os.Open(path)
	if err != nil {
		return Page{}, fmt.Errorf("failed to open page with %w", err)
	}
	defer file.Close()

	info, err := FindPageInfo(file)
	if err != nil {
		return Page{}, fmt.Errorf("failed to read page properties with %w", err)
	}

	return Page{
		Path: path,
		Info: info,
	}, nil
}

func (p Page) Title() string {
	fileName := filepath.Base(p.Path)
	return strings.TrimSuffix(fileName, filepath.Ext(fileName))
}

type PageInfo struct {
	Props []Property
	Refs  []string
}

type Property struct {
	Name   string
	Values []string
	Level  string
}

func (p PageInfo) AllTags() ([]string, bool) {
	for _, p := range p.Props {
		if p.Name != "tags" {
			continue
		}
		return p.Values, true
	}
	return nil, false
}

func (p PageInfo) PageLevelTags() ([]string, bool) {
	for _, p := range p.Props {
		if p.Name != "tags" || p.Level != PageLevel {
			continue
		}
		return p.Values, true
	}
	return nil, false
}

func (p PageInfo) Get(name string) (values []string, ok bool) {
	for _, p := range p.Props {
		if p.Name != name {
			continue
		}
		return p.Values, true
	}
	return values, false
}

func FindPageInfo(r io.Reader) (PageInfo, error) {
	var props PageInfo
	var propertyLevel string
	pageStart := true

	// Scan lines and extract properties
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		// Page properties delimited by new line
		if line == "" {
			pageStart = false
			continue
		}
		// Collect references
		matches := refRegexp.FindAllStringSubmatchIndex(line, -1)
		for _, match := range matches {
			ref := line[match[2]:match[3]]
			// Collect only unique references
			if slices.Contains(props.Refs, ref) {
				continue
			}
			props.Refs = append(props.Refs, ref)
		}

		// Is there any properties
		match := propertyRegexp.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}

		// Found property
		propertyName := line[match[2]:match[3]]

		// Split by comma & trim spaces
		propertyValues := strings.Split(line[match[4]:match[5]], ",")
		for idx, value := range propertyValues {
			propertyValues[idx] = strings.Trim(value, " ")
		}

		// Set property level
		if pageStart {
			propertyLevel = PageLevel
		} else {
			propertyLevel = BlockLevel
		}

		// TODO: Collects all properties, but may contain
		// elements with equal Name and differenc Values
		props.Props = append(props.Props, Property{
			Name:   propertyName,
			Values: propertyValues,
			Level:  propertyLevel,
		})
	}

	return props, nil
}

func ExtractReference(ref string) string {
	return strings.TrimPrefix(strings.TrimSuffix(ref, "]]"), "[[")
}
