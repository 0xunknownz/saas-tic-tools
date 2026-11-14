package reportarchive

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DecodeRuleConfig parses a constrained declarative rule. Unknown keys are rejected so a typo
// cannot silently change extraction, and no field can contain executable code.
func DecodeRuleConfig(raw []byte) (RuleConfig, error) {
	var cfg RuleConfig
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if dec.More() {
		return cfg, fmt.Errorf("%w: trailing JSON", ErrInvalidInput)
	}
	if err := validateRuleConfig(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validateRuleConfig(cfg RuleConfig) error {
	cfg.TemplateKey = strings.TrimSpace(cfg.TemplateKey)
	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.Institution = strings.TrimSpace(cfg.Institution)
	if cfg.TemplateKey == "" || len([]rune(cfg.TemplateKey)) > 80 {
		return fmt.Errorf("%w: templateKey must contain 1-80 characters", ErrInvalidInput)
	}
	if cfg.Name == "" || len([]rune(cfg.Name)) > 120 {
		return fmt.Errorf("%w: name must contain 1-120 characters", ErrInvalidInput)
	}
	if cfg.Institution == "" || len([]rune(cfg.Institution)) > 200 {
		return fmt.Errorf("%w: institution must contain 1-200 characters", ErrInvalidInput)
	}
	if len(cfg.LayoutMarkers) == 0 || len(cfg.LayoutMarkers) > 20 {
		return fmt.Errorf("%w: layoutMarkers must contain 1-20 entries", ErrInvalidInput)
	}
	for _, m := range cfg.LayoutMarkers {
		if strings.TrimSpace(m) == "" || len([]rune(m)) > 200 {
			return fmt.Errorf("%w: invalid layout marker", ErrInvalidInput)
		}
	}
	if len(cfg.Fields) == 0 || len(cfg.Fields) > 30 {
		return fmt.Errorf("%w: fields must contain 1-30 entries", ErrInvalidInput)
	}
	for name, f := range cfg.Fields {
		if len([]rune(name)) == 0 || len([]rune(name)) > 60 {
			return fmt.Errorf("%w: invalid field name", ErrInvalidInput)
		}
		if len(f.Labels) == 0 || len(f.Labels) > 10 {
			return fmt.Errorf("%w: field %s needs 1-10 labels", ErrInvalidInput, name)
		}
		if _, err := compilePattern(f.Pattern); err != nil {
			return fmt.Errorf("%w: field %s: %v", ErrInvalidInput, name, err)
		}
		if f.Format != "" && f.Format != "date" {
			if _, err := compilePattern(f.Format); err != nil {
				return fmt.Errorf("%w: field %s format: %v", ErrInvalidInput, name, err)
			}
		}
		if len(f.Exclude) > 20 {
			return fmt.Errorf("%w: field %s has too many exclusions", ErrInvalidInput, name)
		}
	}
	d := cfg.Details
	if len(d.StartMarkers) > 10 || len(d.EndMarkers) > 10 || len(d.SkipPatterns) > 30 {
		return fmt.Errorf("%w: too many detail markers", ErrInvalidInput)
	}
	if d.RowPattern != "" {
		re, err := compilePattern(d.RowPattern)
		if err != nil {
			return fmt.Errorf("%w: details.rowPattern: %v", ErrInvalidInput, err)
		}
		if re.SubexpIndex("item") < 0 {
			return fmt.Errorf("%w: details.rowPattern must name an item group", ErrInvalidInput)
		}
	}
	if d.SectionPattern != "" {
		if _, err := compilePattern(d.SectionPattern); err != nil {
			return fmt.Errorf("%w: details.sectionPattern: %v", ErrInvalidInput, err)
		}
	}
	if d.ContinuationPattern != "" {
		if _, err := compilePattern(d.ContinuationPattern); err != nil {
			return fmt.Errorf("%w: details.continuationPattern: %v", ErrInvalidInput, err)
		}
	}
	for _, p := range d.SkipPatterns {
		if _, err := compilePattern(p); err != nil {
			return fmt.Errorf("%w: details.skipPatterns: %v", ErrInvalidInput, err)
		}
	}
	if d.MinRows < 0 || d.MinRows > 10000 || d.ExpectedRows < 0 || d.ExpectedRows > 10000 {
		return fmt.Errorf("%w: detail row bounds are invalid", ErrInvalidInput)
	}
	allowed := map[string]bool{"section": true, "item": true, "requirement": true, "unit": true, "method": true, "result": true, "judgement": true}
	for _, f := range d.RequireFields {
		if !allowed[f] {
			return fmt.Errorf("%w: unsupported required detail field %s", ErrInvalidInput, f)
		}
	}
	if len(cfg.Validation.RequiredFields) > 30 || len(cfg.Validation.AllowEmptyValues) > 20 {
		return fmt.Errorf("%w: validation list is too long", ErrInvalidInput)
	}
	// Required fields must exist in the field map; a rule cannot require an extraction it never defines.
	for _, f := range cfg.Validation.RequiredFields {
		if _, ok := cfg.Fields[f]; !ok {
			return fmt.Errorf("%w: required field %s is not defined", ErrInvalidInput, f)
		}
	}
	return nil
}

type compiledRule struct {
	config       RuleConfig
	fields       map[string]*regexp.Regexp
	fieldFormats map[string]*regexp.Regexp
	row          *regexp.Regexp
	section      *regexp.Regexp
	continuation *regexp.Regexp
	skip         []*regexp.Regexp
}

func compileRule(cfg RuleConfig) (*compiledRule, error) {
	c := &compiledRule{config: cfg, fields: map[string]*regexp.Regexp{}, fieldFormats: map[string]*regexp.Regexp{}}
	for name, f := range cfg.Fields {
		re, err := compilePattern(f.Pattern)
		if err != nil {
			return nil, err
		}
		c.fields[name] = re
		if f.Format != "" && f.Format != "date" {
			re, err := compilePattern(f.Format)
			if err != nil {
				return nil, err
			}
			c.fieldFormats[name] = re
		}
	}
	if cfg.Details.RowPattern != "" {
		c.row = regexp.MustCompile(cfg.Details.RowPattern)
	}
	if cfg.Details.SectionPattern != "" {
		c.section = regexp.MustCompile(cfg.Details.SectionPattern)
	}
	if cfg.Details.ContinuationPattern != "" {
		c.continuation = regexp.MustCompile(cfg.Details.ContinuationPattern)
	}
	for _, p := range cfg.Details.SkipPatterns {
		c.skip = append(c.skip, regexp.MustCompile(p))
	}
	return c, nil
}

// chooseRule selects one active rule. Institution and layout markers are both required; a tenant
// rule wins a tie against a built-in rule, while two equally good rules are ambiguous and must go
// to review instead of guessing.
func chooseRule(rules []Rule, text string) (*Rule, []Issue) {
	compact := compactText(text)
	type candidate struct {
		rule  *Rule
		score int
	}
	var candidates []candidate
	for i := range rules {
		r := &rules[i]
		if r.Status != "active" {
			continue
		}
		if r.Config.Institution == "" || !strings.Contains(compact, compactText(r.Config.Institution)) {
			continue
		}
		score := 0
		for _, m := range r.Config.LayoutMarkers {
			if strings.Contains(compact, compactText(m)) {
				score++
			}
		}
		if score == 0 {
			continue
		}
		candidates = append(candidates, candidate{rule: r, score: score})
	}
	if len(candidates) == 0 {
		return nil, []Issue{{Code: "RULE_NOT_MATCHED", Stage: "parse", Message: "未找到匹配的模板规则，需管理员处理"}}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		// Tenant rules (tenant id set) win over built-in templates.
		return candidates[i].rule.Builtin == false && candidates[j].rule.Builtin == true
	})
	top := candidates[0]
	if len(candidates) > 1 && candidates[1].score == top.score && candidates[1].rule.Builtin == top.rule.Builtin {
		return nil, []Issue{{Code: "RULE_AMBIGUOUS", Stage: "parse", Message: "多个模板规则同样匹配，需管理员确认"}}
	}
	return top.rule, nil
}

func compactText(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\u3000' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

type docLine struct {
	text string
	page int
	no   int
}

func documentLines(pages []OCRPage) []docLine {
	var out []docLine
	for _, p := range pages {
		page := p.Page
		if page < 1 {
			page = 1
		}
		for i, line := range strings.Split(strings.ReplaceAll(p.Text, "\r\n", "\n"), "\n") {
			out = append(out, docLine{text: line, page: page, no: i + 1})
		}
	}
	return out
}

// parseDocument extracts fields and details using a compiled rule. It never executes document
// content: every value is treated as text and every pattern is a bounded RE2 expression.
func parseDocument(rule *Rule, pages []OCRPage) ParseResult {
	res := ParseResult{Fields: map[string]string{}, FieldSources: map[string]FieldSource{}, RawFields: map[string]any{},
		Details: []Detail{}, Issues: []Issue{}, RuleID: rule.ID, RuleVersion: rule.Version, TemplateKey: rule.Config.TemplateKey}
	c, err := compileRule(rule.Config)
	if err != nil {
		res.Issues = append(res.Issues, Issue{Code: "RULE_INVALID", Stage: "parse", Message: err.Error()})
		return res
	}
	lines := documentLines(pages)

	// Fields
	for name, fr := range rule.Config.Fields {
		re := c.fields[name]
		if re == nil {
			continue
		}
		found := false
		for _, line := range lines {
			if excluded(line.text, fr.Exclude) || !containsAny(line.text, fr.Labels) {
				continue
			}
			m := re.FindStringSubmatch(line.text)
			if m == nil {
				continue
			}
			value := matchValue(m)
			if value == "" {
				continue
			}
			value = cleanFieldValue(value)
			if fr.Format == "date" {
				d, derr := normalizeDate(value)
				if derr != nil {
					res.Issues = append(res.Issues, Issue{Code: "FIELD_FORMAT_INVALID", Stage: "parse", Field: name, Page: line.page, Line: line.no, Message: derr.Error()})
					continue
				}
				value = d
			} else if re, ok := c.fieldFormats[name]; ok && !re.MatchString(value) {
				res.Issues = append(res.Issues, Issue{Code: "FIELD_FORMAT_INVALID", Stage: "parse", Field: name, Page: line.page, Line: line.no, Message: "字段格式不符合规则"})
				continue
			}
			res.Fields[name] = value
			res.RawFields[name] = strings.TrimSpace(line.text)
			res.FieldSources[name] = FieldSource{Raw: strings.TrimSpace(line.text), Page: line.page, Line: line.no}
			found = true
			break
		}
		if !found {
			if fr.Required {
				res.Issues = append(res.Issues, Issue{Code: "FIELD_MISSING", Stage: "parse", Field: name, Message: "必填字段未识别：" + name})
			}
			continue
		}
	}
	// The report name is generated from the product name; an administrator may correct it later.
	if strings.TrimSpace(res.Fields["reportName"]) == "" && strings.TrimSpace(res.Fields["productName"]) != "" {
		res.Fields["reportName"] = strings.TrimSpace(res.Fields["productName"]) + "检测报告"
	}
	if strings.TrimSpace(res.Fields["reportNumber"]) != "" {
		res.Fields["reportNumber"] = strings.ToUpper(strings.TrimSpace(res.Fields["reportNumber"]))
	}

	// Details
	if rule.Config.Details.RowPattern == "" || len(rule.Config.Details.StartMarkers) == 0 {
		if rule.Config.Details.MinRows > 0 || rule.Config.Validation.RequireDetails {
			res.Issues = append(res.Issues, Issue{Code: "DETAILS_NOT_CONFIGURED", Stage: "parse", Message: "模板未配置检测明细规则"})
		}
		return res
	}
	start := -1
	for i, line := range lines {
		if containsAny(line.text, rule.Config.Details.StartMarkers) {
			start = i
			break
		}
	}
	if start < 0 {
		res.Issues = append(res.Issues, Issue{Code: "DETAILS_SECTION_MISSING", Stage: "parse", Message: "未找到检测明细区域"})
		return res
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if containsAny(lines[i].text, rule.Config.Details.EndMarkers) {
			// A marker in the remark field after the table is the end; the first marker inside the
			// column header is skipped by the start offset.
			end = i
			break
		}
	}
	section := ""
	var prev *Detail
	for i := start + 1; i < end; i++ {
		line := lines[i]
		norm := strings.Join(strings.Fields(line.text), " ")
		if norm == "" {
			continue
		}
		if matchesAny(c.skip, norm) {
			continue
		}
		if c.section != nil {
			if m := c.section.FindStringSubmatch(norm); m != nil {
				section = strings.TrimSpace(matchGroup(m, c.section, "section"))
				continue
			}
		}
		if m := c.row.FindStringSubmatch(norm); m != nil {
			d := Detail{
				Section:     firstNonEmpty(matchGroup(m, c.row, "section"), section),
				ItemName:    cleanFieldValue(matchGroup(m, c.row, "item")),
				Requirement: cleanFieldValue(matchGroup(m, c.row, "requirement")),
				Unit:        cleanFieldValue(matchGroup(m, c.row, "unit")),
				Method:      cleanFieldValue(matchGroup(m, c.row, "method")),
				ResultRaw:   cleanFieldValue(matchGroup(m, c.row, "result")),
				Judgement:   cleanFieldValue(matchGroup(m, c.row, "judgement")),
				Position:    len(res.Details),
				SourcePage:  line.page,
				SourceText:  strings.TrimSpace(line.text),
			}
			if d.Section == "" {
				d.Section = section
			}
			// Merged cells: a blank cell inherits the last row's value in the same column. This is
			// how a multi-row technical requirement stays attached to every sub-item.
			if prev != nil {
				if d.Requirement == "" {
					d.Requirement = prev.Requirement
				}
				if d.Unit == "" {
					d.Unit = prev.Unit
				}
				if d.Method == "" {
					d.Method = prev.Method
				}
				if d.Section == "" {
					d.Section = prev.Section
				}
			}
			res.Details = append(res.Details, d)
			prev = &res.Details[len(res.Details)-1]
			continue
		}
		if c.continuation != nil {
			if m := c.continuation.FindStringSubmatch(norm); m != nil && prev != nil {
				extra := cleanFieldValue(matchGroup(m, c.continuation, "continuation"))
				if extra != "" {
					if prev.ResultRaw == "" {
						prev.ResultRaw = extra
					} else {
						prev.ResultRaw += " " + extra
					}
					prev.SourceText += " " + strings.TrimSpace(line.text)
				}
				continue
			}
		}
		if looksLikeDataLine(norm) {
			res.Issues = append(res.Issues, Issue{Code: "UNPARSED_DETAIL_LINE", Stage: "parse", Page: line.page, Line: line.no,
				Message: "明细行无法按模板列结构解析：" + truncate(norm, 120)})
		}
	}
	if len(res.Details) < rule.Config.Details.MinRows {
		res.Issues = append(res.Issues, Issue{Code: "DETAIL_ROWS_MISSING", Stage: "parse", Message: fmt.Sprintf("检测明细仅 %d 行，少于模板要求的 %d 行", len(res.Details), rule.Config.Details.MinRows)})
	}
	if rule.Config.Details.ExpectedRows > 0 && len(res.Details) != rule.Config.Details.ExpectedRows {
		res.Issues = append(res.Issues, Issue{Code: "DETAIL_ROW_COUNT_MISMATCH", Stage: "parse", Message: fmt.Sprintf("检测明细 %d 行，模板预期 %d 行", len(res.Details), rule.Config.Details.ExpectedRows)})
	}
	for i := range res.Details {
		d := &res.Details[i]
		for _, f := range rule.Config.Details.RequireFields {
			var v string
			switch f {
			case "section":
				v = d.Section
			case "item":
				v = d.ItemName
			case "requirement":
				v = d.Requirement
			case "unit":
				v = d.Unit
			case "method":
				v = d.Method
			case "result":
				v = d.ResultRaw
			case "judgement":
				v = d.Judgement
			}
			if strings.TrimSpace(v) == "" {
				res.Issues = append(res.Issues, Issue{Code: "DETAIL_FIELD_MISSING", Stage: "parse", Message: fmt.Sprintf("第 %d 条明细缺少 %s", i+1, f)})
			}
		}
		if len(res.Issues) > 200 {
			break
		}
	}
	return res
}

func matchValue(m []string) string {
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	if len(m) == 1 {
		return strings.TrimSpace(m[0])
	}
	return ""
}

func matchGroup(m []string, re *regexp.Regexp, name string) string {
	i := re.SubexpIndex(name)
	if i < 0 || i >= len(m) {
		return ""
	}
	return strings.TrimSpace(m[i])
}

func containsAny(s string, needles []string) bool {
	for _, n := range needles {
		if n != "" && strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func excluded(s string, needles []string) bool { return containsAny(s, needles) }

func matchesAny(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

func cleanFieldValue(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, ":：")
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "；;，, ")
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func looksLikeDataLine(s string) bool {
	return regexp.MustCompile(`[0-9]|未检出|合格|不合格`).MatchString(s) && !strings.Contains(s, "第 ") && !strings.Contains(s, "共 ")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
