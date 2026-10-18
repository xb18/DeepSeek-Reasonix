// Package rules loads rule files: markdown a person keeps beside a project (or
// in their home) that the model should follow. A rule with no `paths:` applies
// everywhere; one with `paths:` globs applies once the session has touched a
// matching file. The layout and the frontmatter follow the shape other agents
// use, so a rules directory written for one works in another.
package rules

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	fileencoding "reasonix/internal/base/fileutil/encoding"
	"reasonix/internal/base/frontmatter"
	"reasonix/internal/ext/skill"
)

// Bounds that keep a rules directory from filling a request: one file, one layer
// (unconditional or path-scoped) and the walk itself.
const (
	MaxFileBytes  = 32 * 1024
	MaxLayerBytes = 64 * 1024
	maxFiles      = 200
	maxDepth      = 6
)

// Diagnostic codes. They are identities, so the host can tell the cases apart
// without reading the message.
const (
	CodeInvalidGlob = "rules.invalid_glob"
	CodeTooLarge    = "rules.too_large"
	CodeLayerCap    = "rules.layer_cap"
	CodeTooMany     = "rules.too_many"
	CodeUnreadable  = "rules.unreadable"
)

// Scope is whose rule it is.
type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

// Rule is one rule file.
type Rule struct {
	Path  string
	Label string
	Scope Scope
	Body  string
	// Globs are the usable `paths:` patterns; Gated is true when the file
	// declared any, so a declaration whose every pattern was refused still
	// gates, and a typo hides the rule rather than freeing it.
	Globs []string
	Gated bool
}

// Diagnostic is something the host reports about a rules directory; it never
// reaches the model.
type Diagnostic struct {
	Code    string
	Path    string
	Message string
}

// Result is what Load found, broad to specific: the user's rules first, then the
// project's, so a later rule is the more specific one.
type Result struct {
	Always      []Rule
	Scoped      []Rule
	Diagnostics []Diagnostic
}

// Options say where to look. An empty UserDir skips the user's rules.
type Options struct {
	WorkspaceRoot string
	UserDir       string
}

type source struct {
	dir   string
	label string
	scope Scope
}

// Load reads every rule under the user's rules directory and the project's
// `.claude/rules` and `.reasonix/rules`. It reads the disk each time: what is
// delivered to the model is compared against it, so a rule edited by any means
// is noticed.
func Load(opts Options) Result {
	var sources []source
	if strings.TrimSpace(opts.UserDir) != "" {
		sources = append(sources, source{filepath.Join(opts.UserDir, "rules"), "user/rules", ScopeUser})
	}
	if root := strings.TrimSpace(opts.WorkspaceRoot); root != "" {
		sources = append(sources,
			source{filepath.Join(root, ".claude", "rules"), "workspace/.claude/rules", ScopeProject},
			source{filepath.Join(root, ".reasonix", "rules"), "workspace/.reasonix/rules", ScopeProject})
	}
	var out Result
	var used = map[bool]int{}
	files := 0
	for _, src := range sources {
		for _, path := range markdownFiles(src.dir, &out) {
			if files++; files > maxFiles {
				out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: CodeTooMany, Path: src.dir, Message: "more rule files than the limit; the rest were not read"})
				return out
			}
			rule, ok := read(src, path, &out)
			if !ok {
				continue
			}
			if used[rule.Gated]+len(rule.Body) > MaxLayerBytes {
				out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: CodeLayerCap, Path: path, Message: "dropped: this layer of rules is at its size cap"})
				continue
			}
			used[rule.Gated] += len(rule.Body)
			if rule.Gated {
				out.Scoped = append(out.Scoped, rule)
			} else {
				out.Always = append(out.Always, rule)
			}
		}
	}
	return out
}

// markdownFiles lists the regular .md files under dir in a stable order. A
// symlink is never followed: a rule is text the person keeps in the project, and
// a link out of it could pull a file from anywhere into the prompt.
func markdownFiles(dir string, out *Result) []string {
	var paths []string
	rootDepth := strings.Count(filepath.Clean(dir), string(filepath.Separator))
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == dir && os.IsNotExist(err) {
				return filepath.SkipAll
			}
			out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: CodeUnreadable, Path: path, Message: err.Error()})
			return nil
		}
		if d.IsDir() {
			if strings.Count(path, string(filepath.Separator))-rootDepth >= maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && strings.EqualFold(filepath.Ext(path), ".md") {
			paths = append(paths, path)
		}
		return nil
	})
	slices.Sort(paths)
	return paths
}

func read(src source, path string, out *Result) (Rule, bool) {
	info, err := os.Stat(path)
	if err != nil {
		out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: CodeUnreadable, Path: path, Message: err.Error()})
		return Rule{}, false
	}
	if info.Size() > MaxFileBytes {
		out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: CodeTooLarge, Path: path, Message: "skipped: larger than the per-file limit"})
		return Rule{}, false
	}
	text, err := fileencoding.ReadFileUTF8(path)
	if err != nil {
		out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: CodeUnreadable, Path: path, Message: err.Error()})
		return Rule{}, false
	}
	doc, body := frontmatter.Parse(string(text))
	body = strings.TrimSpace(body)
	if body == "" {
		return Rule{}, false
	}
	rel, _ := filepath.Rel(filepath.Dir(src.dir), path)
	rule := Rule{Path: path, Scope: src.scope, Body: body, Label: src.label + "/" + strings.TrimPrefix(filepath.ToSlash(rel), "rules/")}
	if raw := pathsValue(doc); raw != "" {
		var invalid []string
		rule.Gated = true
		rule.Globs, invalid = skill.ParsePaths(raw)
		if len(invalid) > 0 {
			out.Diagnostics = append(out.Diagnostics, Diagnostic{Code: CodeInvalidGlob, Path: path, Message: "ignored patterns: " + strings.Join(invalid, ", ")})
		}
	}
	return rule, true
}

// Block renders the unconditional rules for the model under one heading, broad
// to specific. It is empty for none, so a turn with nothing to say carries
// nothing.
func Block(rules []Rule) string {
	return render("# Rules", "Rules kept for this project and its user. Treat them as standing rules, not conversation; the most recent block supersedes an earlier one. Later entries are more specific and take precedence when rules conflict; the current user request still has the highest priority.", rules)
}

// ScopedBlock renders the rules whose `paths:` the session has touched.
func ScopedBlock(rules []Rule) string {
	return render("# Path-scoped rules", "Rules that apply to the files they name, which this session has touched. Treat them as standing rules, not conversation; the most recent block supersedes an earlier one. Later entries are more specific and take precedence when rules conflict; the current user request still has the highest priority.", rules)
}

func render(title, intro string, rules []Rule) string {
	if len(rules) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(title + "\n\n" + intro + "\n")
	for _, r := range rules {
		b.WriteString("\n## " + r.Label + " (" + string(r.Scope))
		if len(r.Globs) > 0 {
			b.WriteString(", applies to " + strings.Join(r.Globs, ", "))
		}
		b.WriteString(")\n\n" + r.Body + "\n")
	}
	return strings.TrimSpace(b.String())
}

// pathsValue is the `paths:` declaration as one string for skill.ParsePaths: a
// scalar as written, a sequence joined by commas. An absent or empty declaration
// is "", which leaves the rule unconditional.
func pathsValue(doc frontmatter.Document) string {
	v, ok := doc.Lookup("paths")
	if !ok {
		return ""
	}
	if v.Kind == frontmatter.KindScalar {
		return strings.TrimSpace(v.Scalar)
	}
	var items []string
	for _, item := range v.Items {
		if item.Kind == frontmatter.KindScalar && strings.TrimSpace(item.Scalar) != "" {
			items = append(items, strings.TrimSpace(item.Scalar))
		}
	}
	return strings.Join(items, ", ")
}
