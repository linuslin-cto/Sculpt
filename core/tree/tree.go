// Copyright 2026 Google LLC
// Licensed under the Apache License, Version 2.0

// Package tree implements the c1..c8 component tree: read, validate, write, diff.
package tree

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Tree is relative path -> content for editable harness files.
type Tree map[string]string

// ReservedScripts are framework-provided helpers that proposals need not ship.
var ReservedScripts = map[string]struct{}{
	"read_doc.py": {},
	"pack.py":     {},
	"unpack.py":   {},
}

var nameRE = `[a-z0-9][a-z0-9_\-]*`

// SurfaceSpec describes the editable surface of one agent framework.
type SurfaceSpec struct {
	Name                 string
	AlwaysOn             []string
	AlwaysOnMax          int
	AlwaysOnCapReason    string
	SkillDir             string
	SkillNameRule        string
	TriggerHint          string
	AutotriggerFile      string
	TriggerLine          string
	SubagentRequiresMode bool
	PluginDir            string // empty if unsupported
	HooksFile            string // empty if unsupported
	HookEvents           map[string]struct{}
	CompileSkillScripts  bool
	RejectEmptyAlwaysOn  bool
	Stubs                [][2]string // rel, body
}

type patterns struct {
	skill       *regexp.Regexp
	skillScript *regexp.Regexp
	subagent    *regexp.Regexp
	script      *regexp.Regexp
	scriptRef   *regexp.Regexp
	plugin      *regexp.Regexp
}

func (s SurfaceSpec) rx() patterns {
	sd := regexp.QuoteMeta(s.SkillDir)
	p := patterns{
		skill:       regexp.MustCompile(`^` + sd + `/(` + nameRE + `)/SKILL\.md$`),
		skillScript: regexp.MustCompile(`^` + sd + `/` + nameRE + `/scripts/[A-Za-z0-9_.\-]+$`),
		subagent:    regexp.MustCompile(`^sub_agents/(` + nameRE + `)/AGENT\.md$`),
		script:      regexp.MustCompile(`^scripts/[a-z0-9_][a-z0-9_]*\.py$`),
		scriptRef:   regexp.MustCompile(`(?:python3?\s+)((?:` + sd + `/[\w\-]+/)?scripts/[\w.\-]+\.py)`),
	}
	if s.PluginDir != "" {
		p.plugin = regexp.MustCompile(`^` + regexp.QuoteMeta(s.PluginDir) + `/(` + nameRE + `)\.ts$`)
	}
	return p
}

// NormalizeRel normalises a relative path, rejecting escapes.
func NormalizeRel(rel string) (string, error) {
	r := strings.TrimSpace(strings.ReplaceAll(rel, `\`, "/"))
	if strings.HasPrefix(r, "/") {
		return "", fmt.Errorf("path escapes the harness: %q", rel)
	}
	for _, part := range strings.Split(r, "/") {
		if part == ".." {
			return "", fmt.Errorf("path escapes the harness: %q", rel)
		}
	}
	for strings.HasPrefix(r, "./") {
		r = r[2:]
	}
	return strings.Trim(r, "/"), nil
}

// IsAllowed reports whether rel is on the editable surface.
func IsAllowed(surface SurfaceSpec, rel string) bool {
	norm, err := NormalizeRel(rel)
	if err != nil {
		return false
	}
	for _, a := range surface.AlwaysOn {
		if norm == a {
			return true
		}
	}
	if surface.HooksFile != "" && norm == surface.HooksFile {
		return true
	}
	r := surface.rx()
	if r.skill.MatchString(norm) || r.skillScript.MatchString(norm) ||
		r.subagent.MatchString(norm) || r.script.MatchString(norm) {
		return true
	}
	return r.plugin != nil && r.plugin.MatchString(norm)
}

func stubPaths(surface SurfaceSpec) map[string]struct{} {
	out := make(map[string]struct{}, len(surface.Stubs))
	for _, s := range surface.Stubs {
		out[s[0]] = struct{}{}
	}
	return out
}

// SkillNames returns sorted skill directory names present in tree.
func SkillNames(surface SurfaceSpec, t Tree) []string {
	r := surface.rx()
	set := map[string]struct{}{}
	for rel := range t {
		if m := r.skill.FindStringSubmatch(rel); m != nil {
			set[m[1]] = struct{}{}
		}
	}
	return sortedKeys(set)
}

// SubagentNames returns sorted subagent names present in tree.
func SubagentNames(surface SurfaceSpec, t Tree) []string {
	r := surface.rx()
	set := map[string]struct{}{}
	for rel := range t {
		if m := r.subagent.FindStringSubmatch(rel); m != nil {
			set[m[1]] = struct{}{}
		}
	}
	return sortedKeys(set)
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ReadTree loads editable files under d. Rejected off-surface paths append to rejected.
func ReadTree(surface SurfaceSpec, d string, rejected *[]string) Tree {
	out := Tree{}
	info, err := os.Stat(d)
	if err != nil || !info.IsDir() {
		return out
	}
	stubs := stubPaths(surface)
	_ = filepath.WalkDir(d, func(path string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(d, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.Contains(rel, "__pycache__") {
			return nil
		}
		if _, ok := stubs[rel]; ok {
			return nil
		}
		if !IsAllowed(surface, rel) {
			if rejected != nil {
				*rejected = append(*rejected, rel)
			}
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		out[rel] = string(b)
		return nil
	})
	return out
}

// IsTree reports whether d looks like a harness (any always-on file present).
func IsTree(surface SurfaceSpec, d string) bool {
	info, err := os.Stat(d)
	if err != nil || !info.IsDir() {
		return false
	}
	for _, a := range surface.AlwaysOn {
		if fi, err := os.Stat(filepath.Join(d, filepath.FromSlash(a))); err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
}

// WriteTree materialises an exact editable tree into dest.
func WriteTree(surface SurfaceSpec, t Tree, dest string, stubs bool) (string, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	stubSet := stubPaths(surface)
	var files []string
	_ = filepath.WalkDir(dest, func(path string, de os.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dest, path)
		rel = filepath.ToSlash(rel)
		files = append(files, rel)
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for _, rel := range files {
		if IsAllowed(surface, rel) {
			_ = os.Remove(filepath.Join(dest, filepath.FromSlash(rel)))
		} else if _, ok := stubSet[rel]; ok {
			_ = os.Remove(filepath.Join(dest, filepath.FromSlash(rel)))
		}
	}
	for _, a := range surface.AlwaysOn {
		p := filepath.Join(dest, filepath.FromSlash(a))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if _, ok := t[a]; !ok {
			if _, err := os.Stat(p); os.IsNotExist(err) {
				_ = os.WriteFile(p, []byte{}, 0o644)
			}
		}
	}
	for rel, content := range t {
		norm, err := NormalizeRel(rel)
		if err != nil {
			return "", err
		}
		p := filepath.Join(dest, filepath.FromSlash(norm))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	if stubs {
		for _, s := range surface.Stubs {
			p := filepath.Join(dest, filepath.FromSlash(s[0]))
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			if _, err := os.Stat(p); os.IsNotExist(err) {
				_ = os.WriteFile(p, []byte(s[1]), 0o644)
			}
		}
	}
	return dest, nil
}

// AlwaysOnText concatenates always-on file bodies.
func AlwaysOnText(surface SurfaceSpec, t Tree) string {
	parts := make([]string, 0, len(surface.AlwaysOn))
	for _, a := range surface.AlwaysOn {
		parts = append(parts, t[a])
	}
	return strings.Join(parts, "\n\n")
}

// Autotrigger adds trigger lines for unmentioned skills; returns repaired names.
func Autotrigger(surface SurfaceSpec, t Tree, alwaysOnMax int) []string {
	capN := surface.AlwaysOnMax
	if alwaysOnMax > 0 {
		capN = alwaysOnMax
	}
	body := AlwaysOnText(surface, t)
	var fixed []string
	descRE := regexp.MustCompile(`(?m)^description:\s*(.+)$`)
	for _, name := range SkillNames(surface, t) {
		if strings.Contains(body, name) {
			continue
		}
		fm := t[surface.SkillDir+"/"+name+"/SKILL.md"]
		if len(fm) > 600 {
			fm = fm[:600]
		}
		desc := fmt.Sprintf("the %s workflow applies", name)
		if m := descRE.FindStringSubmatch(fm); m != nil {
			desc = strings.TrimRight(strings.TrimSpace(m[1]), ".")
		}
		line := strings.ReplaceAll(surface.TriggerLine, "{name}", name)
		line = strings.ReplaceAll(line, "{desc}", desc)
		cur := t[surface.AutotriggerFile]
		if len(AlwaysOnText(surface, t))+len(line)+2 > capN {
			continue
		}
		if strings.TrimSpace(cur) == "" {
			t[surface.AutotriggerFile] = line
		} else {
			t[surface.AutotriggerFile] = strings.TrimRight(cur, "\n") + "\n" + line
		}
		body = AlwaysOnText(surface, t)
		fixed = append(fixed, name)
	}
	return fixed
}

// DiffSummary compares before/after trees.
func DiffSummary(surface SurfaceSpec, before, after Tree) map[string]any {
	ba, br, bm := []string{}, []string{}, []string{}
	for rel := range after {
		if _, ok := before[rel]; !ok {
			ba = append(ba, rel)
		}
	}
	for rel := range before {
		if _, ok := after[rel]; !ok {
			br = append(br, rel)
		} else if before[rel] != after[rel] {
			bm = append(bm, rel)
		}
	}
	sort.Strings(ba)
	sort.Strings(br)
	sort.Strings(bm)
	return map[string]any{
		"added":               ba,
		"removed":             br,
		"modified":            bm,
		"always_on_char_delta": len(AlwaysOnText(surface, after)) - len(AlwaysOnText(surface, before)),
	}
}

// Census is returned by Validate on success.
type Census struct {
	AlwaysOnChars int `json:"always_on_chars"`
	NFiles        int `json:"n_files"`
	Skills        int `json:"skills"`
	SubAgents     int `json:"sub_agents"`
	Scripts       int `json:"scripts"`
	Plugins       int `json:"plugins,omitempty"`
	Hooks         int `json:"hooks,omitempty"`
}

// Validate repairs checks and returns a census, or an error listing every problem.
func Validate(surface SurfaceSpec, t Tree, allowSubagents bool, alwaysOnMax int, reserved map[string]struct{}) (Census, error) {
	if reserved == nil {
		reserved = ReservedScripts
	}
	var errs []string
	r := surface.rx()
	capN := surface.AlwaysOnMax
	if alwaysOnMax > 0 {
		capN = alwaysOnMax
	}
	body := AlwaysOnText(surface, t)
	if len(body) > capN {
		errs = append(errs, fmt.Sprintf("always-on content is %d chars, over the %d cap; %s",
			len(body), capN, surface.AlwaysOnCapReason))
	}
	if surface.RejectEmptyAlwaysOn && strings.TrimSpace(body) == "" {
		errs = append(errs, fmt.Sprintf("%s is empty — it is the only always-on file on this framework, so an empty one is a harness that does not exist", surface.AlwaysOn[0]))
	}
	skills := checkSkills(surface, t, body, &errs)
	subs := checkSubagents(surface, t, allowSubagents, &errs)
	for rel, src := range t {
		if r.script.MatchString(rel) || (surface.CompileSkillScripts && r.skillScript.MatchString(rel) && strings.HasSuffix(rel, ".py")) {
			if err := checkPythonCompiles(rel, src); err != "" {
				errs = append(errs, err)
			}
		}
	}
	if surface.HooksFile != "" {
		if src, ok := t[surface.HooksFile]; ok {
			checkHooks(surface, src, &errs)
		}
	}
	checkScriptRefs(surface, t, r, reserved, &errs)
	if len(errs) > 0 {
		return Census{}, fmt.Errorf("%s", strings.Join(errs, " | "))
	}
	c := Census{
		AlwaysOnChars: len(body),
		NFiles:        len(t),
		Skills:        len(skills),
		SubAgents:     len(subs),
	}
	for rel := range t {
		if r.script.MatchString(rel) {
			c.Scripts++
		}
		if r.plugin != nil && r.plugin.MatchString(rel) {
			c.Plugins++
		}
	}
	if surface.HooksFile != "" {
		if _, ok := t[surface.HooksFile]; ok {
			c.Hooks = 1
		}
	}
	return c, nil
}

func checkSkills(surface SurfaceSpec, t Tree, body string, errs *[]string) []string {
	skills := SkillNames(surface, t)
	descRE := regexp.MustCompile(`(?m)^description:\s*\S`)
	nameRE2 := regexp.MustCompile(`(?m)^name:\s*(\S+)\s*$`)
	for _, name := range skills {
		fm := t[surface.SkillDir+"/"+name+"/SKILL.md"]
		if len(fm) > 600 {
			fm = fm[:600]
		}
		if !descRE.MatchString(fm) {
			*errs = append(*errs, fmt.Sprintf("%s/%s/SKILL.md needs a `description:` in its frontmatter, or the skill is dropped SILENTLY at load", surface.SkillDir, name))
		}
		m := nameRE2.FindStringSubmatch(fm)
		if surface.SkillNameRule == "must_match" {
			if m == nil || m[1] != name {
				*errs = append(*errs, fmt.Sprintf("%s/%s/SKILL.md needs frontmatter `name: %s` matching its directory (a mismatch is dropped SILENTLY at load)", surface.SkillDir, name, name))
			}
		} else if m != nil && m[1] != name {
			*errs = append(*errs, fmt.Sprintf("%s/%s/SKILL.md frontmatter `name: %s` OVERRIDES the directory name at load", surface.SkillDir, name, m[1]))
		}
		if !strings.Contains(body, name) {
			hint := strings.ReplaceAll(surface.TriggerHint, "{name}", name)
			*errs = append(*errs, fmt.Sprintf("skill '%s' is never named in the always-on text, so the agent will not know to invoke it — an untriggered skill is dead weight. %s", name, hint))
		}
	}
	return skills
}

func checkSubagents(surface SurfaceSpec, t Tree, allow bool, errs *[]string) []string {
	subs := SubagentNames(surface, t)
	if len(subs) > 0 && !allow {
		*errs = append(*errs, fmt.Sprintf("sub_agents are not available on this deployment: %v", subs))
	}
	modeRE := regexp.MustCompile(`(?m)^mode:\s*subagent\s*$`)
	descRE := regexp.MustCompile(`(?m)^description:\s*\S`)
	for _, name := range subs {
		fm := t["sub_agents/"+name+"/AGENT.md"]
		if len(fm) > 600 {
			fm = fm[:600]
		}
		if surface.SubagentRequiresMode && !modeRE.MatchString(fm) {
			*errs = append(*errs, fmt.Sprintf("sub_agents/%s/AGENT.md needs frontmatter `mode: subagent`", name))
		}
		if !descRE.MatchString(fm) {
			*errs = append(*errs, fmt.Sprintf("sub_agents/%s/AGENT.md needs a `description:`", name))
		}
	}
	return subs
}

func checkPythonCompiles(rel, src string) string {
	if strings.TrimSpace(src) == "" {
		return fmt.Sprintf("%s does not compile: empty script", rel)
	}
	// Prefer the real Python compiler when available.
	for _, bin := range []string{"python", "python3", "py"} {
		args := []string{"-c", "import sys; compile(sys.stdin.read(), sys.argv[1], 'exec')", rel}
		if bin == "py" {
			args = append([]string{"-3"}, args...)
		}
		cmd := exec.Command(bin, args...)
		cmd.Stdin = strings.NewReader(src)
		if out, err := cmd.CombinedOutput(); err != nil {
			detail := strings.TrimSpace(string(out))
			if detail == "" {
				detail = err.Error()
			}
			if len(detail) > 160 {
				detail = detail[:160]
			}
			return fmt.Sprintf("%s does not compile: %s", rel, detail)
		}
		return ""
	}
	return ""
}

func checkHooks(surface SurfaceSpec, src string, errs *[]string) {
	hf := surface.HooksFile
	var doc map[string]any
	if err := json.Unmarshal([]byte(src), &doc); err != nil {
		*errs = append(*errs, fmt.Sprintf("%s is not valid JSON: %v", hf, err))
		return
	}
	hooks, _ := doc["hooks"].(map[string]any)
	if len(hooks) == 0 {
		*errs = append(*errs, fmt.Sprintf(`%s needs a top-level {"hooks": {"<Event>": [...]}} object`, hf))
		return
	}
	for event, entries := range hooks {
		if _, ok := surface.HookEvents[event]; !ok {
			keys := make([]string, 0, len(surface.HookEvents))
			for k := range surface.HookEvents {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			*errs = append(*errs, fmt.Sprintf("%s: unknown event %q; the framework dispatches %s (PascalCase)", hf, event, strings.Join(keys, ", ")))
			continue
		}
		list, ok := entries.([]any)
		if !ok {
			*errs = append(*errs, fmt.Sprintf("%s: %s must map to a LIST of matcher entries", hf, event))
			continue
		}
		for i, entry := range list {
			em, ok := entry.(map[string]any)
			if !ok {
				*errs = append(*errs, fmt.Sprintf("%s: %s[%d] needs a `hooks` list", hf, event, i))
				continue
			}
			hl, ok := em["hooks"].([]any)
			if !ok {
				*errs = append(*errs, fmt.Sprintf("%s: %s[%d] needs a `hooks` list", hf, event, i))
				continue
			}
			for _, h := range hl {
				hm, _ := h.(map[string]any)
				if hm == nil || hm["type"] != "command" {
					*errs = append(*errs, fmt.Sprintf(`%s: %s[%d] hook type must be "command"`, hf, event, i))
				} else if strings.TrimSpace(fmt.Sprint(hm["command"])) == "" {
					*errs = append(*errs, fmt.Sprintf("%s: %s[%d] has an empty `command`", hf, event, i))
				}
			}
		}
	}
}

func checkScriptRefs(surface SurfaceSpec, t Tree, r patterns, reserved map[string]struct{}, errs *[]string) {
	have := map[string]struct{}{}
	for rel := range t {
		if r.script.MatchString(rel) || r.skillScript.MatchString(rel) {
			have[rel] = struct{}{}
		}
	}
	always := map[string]struct{}{}
	for _, a := range surface.AlwaysOn {
		always[a] = struct{}{}
	}
	for rel, src := range t {
		if !(r.skill.MatchString(rel) || func() bool { _, ok := always[rel]; return ok }()) {
			continue
		}
		for _, m := range r.scriptRef.FindAllStringSubmatch(src, -1) {
			ref := m[1]
			base := filepath.Base(ref)
			if _, ok := reserved[base]; ok {
				continue
			}
			if _, ok := have[ref]; !ok {
				*errs = append(*errs, fmt.Sprintf("%s invokes `%s` but that file is not in the harness", rel, ref))
			}
		}
	}
}

// Diversity computes pairwise line-Jaccard over candidate trees.
func Diversity(trees map[string]Tree) map[string]any {
	names := make([]string, 0, len(trees))
	for n := range trees {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) < 2 {
		return map[string]any{"n": len(names), "n_distinct": len(names), "mean_line_jaccard": 1.0}
	}
	lineSets := map[string]map[string]struct{}{}
	for _, n := range names {
		s := map[string]struct{}{}
		for rel, c := range trees[n] {
			for _, ln := range strings.Split(c, "\n") {
				if strings.TrimSpace(ln) == "" {
					continue
				}
				s[rel+":"+ln] = struct{}{}
			}
		}
		lineSets[n] = s
	}
	var sims []float64
	for i, a := range names {
		for _, b := range names[i+1:] {
			inter, union := 0, 0
			seen := map[string]struct{}{}
			for k := range lineSets[a] {
				seen[k] = struct{}{}
				if _, ok := lineSets[b][k]; ok {
					inter++
				}
			}
			for k := range lineSets[b] {
				seen[k] = struct{}{}
			}
			union = len(seen)
			if union == 0 {
				sims = append(sims, 1.0)
			} else {
				sims = append(sims, float64(inter)/float64(union))
			}
		}
	}
	distinct := map[string]struct{}{}
	for _, t := range trees {
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		for _, k := range keys {
			b.WriteString(k)
			b.WriteByte(0)
			b.WriteString(t[k])
			b.WriteByte(0)
		}
		distinct[b.String()] = struct{}{}
	}
	sum := 0.0
	for _, s := range sims {
		sum += s
	}
	mean := float64(int((sum/float64(len(sims)))*10000+0.5)) / 10000
	return map[string]any{"n": len(names), "n_distinct": len(distinct), "mean_line_jaccard": mean}
}

// Clone returns a shallow copy of the tree.
func Clone(t Tree) Tree {
	out := make(Tree, len(t))
	for k, v := range t {
		out[k] = v
	}
	return out
}
