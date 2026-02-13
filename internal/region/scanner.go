package region

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RegionMarker represents a parsed region marker in a source file.
type RegionMarker struct {
	Path      string // namespace path (e.g., "app.search.sources.btv2")
	File      string // source file path
	StartLine int    // line number of @region marker
	EndLine   int    // line number of @endregion marker (0 if unclosed)
	Children  []*RegionMarker
}

// TreeNode represents a node in the region tree view.
type TreeNode struct {
	Name     string
	FullPath string
	File     string
	Start    int
	End      int
	Children []*TreeNode
}

// CommentStyle maps file extensions to their comment prefix.
var CommentStyle = map[string]string{
	".go":     "//",
	".c":      "//",
	".h":      "//",
	".java":   "//",
	".js":     "//",
	".ts":     "//",
	".tsx":    "//",
	".jsx":    "//",
	".rs":     "//",
	".swift":  "//",
	".py":     "#",
	".rb":     "#",
	".sh":     "#",
	".bash":   "#",
	".yaml":   "#",
	".yml":    "#",
	".toml":   "#",
	".sql":    "--",
	".lua":    "--",
	".hs":     "--",
	".css":    "/*",
	".scss":   "/*",
}

// HTMLStyleExtensions use <!-- --> comment syntax.
var HTMLStyleExtensions = map[string]bool{
	".html":   true,
	".xml":    true,
	".vue":    true,
	".svelte": true,
}

// GetCommentPrefix returns the comment prefix for a file extension.
func GetCommentPrefix(filename string) string {
	ext := filepath.Ext(filename)
	if HTMLStyleExtensions[ext] {
		return "<!--"
	}
	if prefix, ok := CommentStyle[ext]; ok {
		return prefix
	}
	return "//"
}

// GetRegionTag returns the region start tag for a given path and file.
func GetRegionTag(path, filename string) string {
	ext := filepath.Ext(filename)
	if HTMLStyleExtensions[ext] {
		return fmt.Sprintf("<!-- @region:%s -->", path)
	}
	prefix := GetCommentPrefix(filename)
	if prefix == "/*" {
		return fmt.Sprintf("/* @region:%s */", path)
	}
	return fmt.Sprintf("%s @region:%s", prefix, path)
}

// GetEndRegionTag returns the endregion tag for a given path and file.
func GetEndRegionTag(path, filename string) string {
	ext := filepath.Ext(filename)
	if HTMLStyleExtensions[ext] {
		return fmt.Sprintf("<!-- @endregion:%s -->", path)
	}
	prefix := GetCommentPrefix(filename)
	if prefix == "/*" {
		return fmt.Sprintf("/* @endregion:%s */", path)
	}
	return fmt.Sprintf("%s @endregion:%s", prefix, path)
}

// ScanFile scans a source file for region markers and returns them.
func ScanFile(filename string) ([]*RegionMarker, []string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var markers []*RegionMarker
	var warnings []string
	openRegions := make(map[string]*RegionMarker)

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if path, ok := extractRegionPath(line, "region"); ok {
			marker := &RegionMarker{
				Path:      path,
				File:      filename,
				StartLine: lineNum,
			}
			openRegions[path] = marker
			markers = append(markers, marker)
		}

		if path, ok := extractRegionPath(line, "endregion"); ok {
			if m, exists := openRegions[path]; exists {
				m.EndLine = lineNum
				delete(openRegions, path)
			} else {
				warnings = append(warnings, fmt.Sprintf(
					"%s:%d: @endregion:%s without matching @region",
					filename, lineNum, path,
				))
			}
		}
	}

	for path, m := range openRegions {
		warnings = append(warnings, fmt.Sprintf(
			"%s:%d: @region:%s never closed",
			filename, m.StartLine, path,
		))
	}

	return markers, warnings, scanner.Err()
}

// extractRegionPath extracts the region path from a line like "// @region:app.search"
func extractRegionPath(line, tag string) (string, bool) {
	marker := "@" + tag + ":"
	idx := strings.Index(line, marker)
	if idx == -1 {
		return "", false
	}
	rest := line[idx+len(marker):]
	// Trim trailing comment closers and whitespace
	rest = strings.TrimRight(rest, " \t")
	rest = strings.TrimSuffix(rest, "-->")
	rest = strings.TrimSuffix(rest, "*/")
	rest = strings.TrimSpace(rest)
	// Take first word as the path
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "", false
	}
	return parts[0], true
}

// ScanDirectory scans all source files in a directory tree for region markers.
func ScanDirectory(dir string, gamignorePatterns []string) ([]*RegionMarker, []string, error) {
	var allMarkers []*RegionMarker
	var allWarnings []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file has a known extension
		ext := filepath.Ext(path)
		if _, ok := CommentStyle[ext]; !ok {
			if !HTMLStyleExtensions[ext] {
				return nil
			}
		}

		// Check gamignore
		relPath, _ := filepath.Rel(dir, path)
		if isIgnored(relPath, gamignorePatterns) {
			return nil
		}

		markers, warnings, err := ScanFile(path)
		if err != nil {
			return nil
		}
		allMarkers = append(allMarkers, markers...)
		allWarnings = append(allWarnings, warnings...)
		return nil
	})

	return allMarkers, allWarnings, err
}

// BuildTree constructs a tree from a flat list of region markers.
func BuildTree(markers []*RegionMarker) *TreeNode {
	root := &TreeNode{Name: "root", FullPath: ""}
	nodeMap := map[string]*TreeNode{"": root}

	// Sort by path to ensure parents exist before children
	sort.Slice(markers, func(i, j int) bool {
		return markers[i].Path < markers[j].Path
	})

	for _, m := range markers {
		parts := strings.Split(m.Path, ".")
		current := root
		for i, part := range parts {
			fullPath := strings.Join(parts[:i+1], ".")
			if child, exists := nodeMap[fullPath]; exists {
				current = child
				continue
			}
			child := &TreeNode{
				Name:     part,
				FullPath: fullPath,
			}
			if fullPath == m.Path {
				child.File = m.File
				child.Start = m.StartLine
				child.End = m.EndLine
			}
			current.Children = append(current.Children, child)
			nodeMap[fullPath] = child
			current = child
		}
	}

	return root
}

// FormatTree produces a text tree view from a TreeNode.
func FormatTree(node *TreeNode, prefix string, isLast bool) string {
	var sb strings.Builder
	if node.Name != "root" {
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		sb.WriteString(prefix + connector + node.Name)
		if node.File != "" {
			sb.WriteString(fmt.Sprintf("    [%s:%d-%d]", node.File, node.Start, node.End))
		}
		sb.WriteString("\n")
	}

	childPrefix := prefix
	if node.Name != "root" {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range node.Children {
		isChildLast := i == len(node.Children)-1
		sb.WriteString(FormatTree(child, childPrefix, isChildLast))
	}

	return sb.String()
}

// FileHasRegionMarkers checks if a file contains region markers for a given path.
func FileHasRegionMarkers(filename, regionPath string) bool {
	markers, _, err := ScanFile(filename)
	if err != nil {
		return false
	}
	for _, m := range markers {
		if m.Path == regionPath {
			return true
		}
	}
	return false
}

// ScaffoldRegion creates or appends region markers in a file.
func ScaffoldRegion(filename, regionPath string) error {
	startTag := GetRegionTag(regionPath, filename)
	endTag := GetEndRegionTag(regionPath, filename)

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// Create new file with region markers
		dir := filepath.Dir(filename)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}

		ext := filepath.Ext(filename)
		var content string
		if ext == ".go" {
			pkg := filepath.Base(filepath.Dir(filename))
			content = fmt.Sprintf("%s\npackage %s\n\n%s\n", startTag, pkg, endTag)
		} else {
			content = fmt.Sprintf("%s\n\n%s\n", startTag, endTag)
		}
		return os.WriteFile(filename, []byte(content), 0644)
	}

	// File exists — check if region already exists
	if FileHasRegionMarkers(filename, regionPath) {
		return nil // already present
	}

	// Append region markers at end of file
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "\n%s\n\n%s\n", startTag, endTag)
	return err
}

// ParseGamignore reads .gamignore patterns from a file.
func ParseGamignore(projectRoot string) []string {
	path := filepath.Join(projectRoot, ".gamignore")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var patterns []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func isIgnored(relPath string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, relPath)
		if err == nil && matched {
			return true
		}
		// Also check if path starts with a directory pattern
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(relPath, strings.TrimSuffix(pattern, "/")) {
				return true
			}
		}
		// Check directory prefix match
		if strings.HasPrefix(relPath, pattern) {
			return true
		}
	}
	return false
}

// ParseArchMd extracts region paths from an arch.md file.
func ParseArchMd(projectRoot string) ([]string, error) {
	archPath := filepath.Join(projectRoot, "arch.md")
	data, err := os.ReadFile(archPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var paths []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if path, ok := extractRegionPath(line, "region"); ok {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// FindUnregionedCode finds source files with code not inside any region markers.
func FindUnregionedCode(dir string, gamignorePatterns []string) ([]string, error) {
	var unregioned []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		ext := filepath.Ext(path)
		if _, ok := CommentStyle[ext]; !ok {
			if !HTMLStyleExtensions[ext] {
				return nil
			}
		}

		relPath, _ := filepath.Rel(dir, path)
		if isIgnored(relPath, gamignorePatterns) {
			return nil
		}

		markers, _, scanErr := ScanFile(path)
		if scanErr != nil {
			return nil
		}

		if len(markers) == 0 {
			unregioned = append(unregioned, relPath)
		}
		return nil
	})

	return unregioned, err
}
