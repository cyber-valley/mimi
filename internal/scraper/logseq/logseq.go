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
)

type RegexGraph struct {
	path string
}

func NewRegexGraph(path string) *RegexGraph {
	return &RegexGraph{
		path: path,
	}
}

func (g RegexGraph) Pages() iter.Seq[Page] {
	return func(yield func(p Page) bool) {
		_ = filepath.Walk(g.path, func(path string, fs fs.FileInfo, err error) error {
			if err != nil {
				slog.Error("failed to access path", "path", path, "with", err)
				return nil
			}

			if strings.HasSuffix(fs.Name(), ".md") {
				return nil
			}

			if !yield(Page{Path: path}) {
				return fmt.Errorf("pages walk iteration stopped")
			}

			return nil
		})
	}
}

type Page struct {
	Path string
}

func (p Page) FindProperties() (Properties, error) {
	// Open page for reading
	file, err := os.Open(p.Path)
	if err != nil {
		return Properties{}, fmt.Errorf("failed to open page with %w", err)
	}
	defer file.Close()

	return FindProperties(file)
}

type Properties struct {
	Result []Property
}

type Property struct {
	Name   string
	Values []string
	Level  string
}

func (p Properties) AllTags() (names []string, ok bool) {
	for _, p := range p.Result {
		if p.Name != "tags" {
			continue
		}
		return slices.Concat(names, p.Values), true
	}
	return names, false
}

func (p Properties) PageLevelTags() (names []string, ok bool) {
	for _, p := range p.Result {
		if p.Name != "tags" || p.Level != "page" {
			continue
		}
		return slices.Concat(names, p.Values), true
	}
	return names, false
}

func (p Properties) Get(name string) (values []string, ok bool) {
	for _, p := range p.Result {
		if p.Name != name {
			continue
		}
		return p.Values, true
	}
	return values, false
}

func FindProperties(r io.Reader) (Properties, error) {
	var props Properties
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
			propertyLevel = "page"
		} else {
			propertyLevel = "block"
		}

		props.Result = append(props.Result, Property{
			Name:   propertyName,
			Values: propertyValues,
			Level:  propertyLevel,
		})
	}

	return props, nil
}
