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

	// Fields. Two passes keep the result deterministic: first the classic "label: value" line,
	// then a bounded look at adjacent lines for table layouts where OCR separates the label and
	// its value. Every candidate is checked against the rule format and the other field values.
	names := make([]string, 0, len(rule.Config.Fields))
	for name := range rule.Config.Fields {
		names = append(names, name)
	}
	// Required fields claim their values first: an optional field must not steal the value that
	// the required field's adjacent lookup is about to use.
	sort.SliceStable(names, func(i, j int) bool {
		ri, rj := rule.Config.Fields[names[i]].Required, rule.Config.Fields[names[j]].Required
		if ri != rj {
			return ri
		}
		return names[i] < names[j]
	})
	allLabels := []string{}
	for _, fr := range rule.Config.Fields {
		allLabels = append(allLabels, fr.Labels...)
	}
	for _, name := range names {
		fr := rule.Config.Fields[name]
		re := c.fields[name]
		if re == nil {
			continue
		}
		for _, line := range lines {
			if excluded(line.text, fr.Exclude) || !containsAny(line.text, fr.Labels) {
				continue
			}
			m := re.FindStringSubmatch(line.text)
			if m == nil {
				continue
			}
			value := matchValue(m)
			if value == "" || labelSuffixValue(fr, value) {
				continue
			}
			// A label printed without a colon and a one/two-character capture is OCR noise
			// ("生产单位Q" -> "Q"), not a field value.
			if !strings.ContainsAny(line.text, ":：") && len([]rune(value)) <= 2 {
				continue
			}
			value, ok := coerceFieldValue(name, fr, value, c)
			if !ok {
				continue
			}
			assignField(&res, name, value, line)
			break
		}
	}
	for _, name := range names {
		if _, ok := res.Fields[name]; ok {
			continue
		}
		fr := rule.Config.Fields[name]
		re := c.fields[name]
		if re == nil {
			continue
		}
		for i, line := range lines {
			if excluded(line.text, fr.Exclude) || !containsAny(line.text, fr.Labels) {
				continue
			}
			if value, src, ok := adjacentFieldValue(name, fr, re, c, lines, i, allLabels, res.Fields); ok {
				assignField(&res, name, value, src)
				break
			}
		}
	}
	// The issuing institution is often only in the page header, not next to a "检测机构" label.
	if strings.TrimSpace(res.Fields["institution"]) == "" && rule.Config.Institution != "" {
		for _, line := range lines {
			if strings.Contains(line.text, rule.Config.Institution) {
				assignField(&res, "institution", rule.Config.Institution, line)
				break
			}
		}
	}
	for _, name := range names {
		if _, ok := res.Fields[name]; !ok && rule.Config.Fields[name].Required {
			res.Issues = append(res.Issues, Issue{Code: "FIELD_MISSING", Stage: "parse", Field: name, Message: "必填字段未识别：" + name})
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
		trimmed := strings.TrimSpace(line.text)
		if startsWithAny(trimmed, rule.Config.Details.StartMarkers) {
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
	startPage, endPage := lines[start].page, lines[start].page
	if end-1 >= start && end-1 < len(lines) {
		endPage = lines[end-1].page
	}
	if details, issues, ok := parseDetailsFromBoxes(rule, pages, startPage, endPage); ok {
		res.Details = append(res.Details, details...)
		res.Issues = append(res.Issues, issues...)
	} else {
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

var detailColumnHeaders = []string{"序号", "检验项目", "技术要求", "单位", "检验方法", "检验结果", "单项判定"}

var detailRowNumberRE = regexp.MustCompile("^[0-9]{1,3}$")

var detailWatermarkRE = regexp.MustCompile("^(?:JXQI?|XQI|QI|TEST|Test Report)$")

var detailNoiseRE = regexp.MustCompile("^(?:JXQI?|XQI|QI|TEST|Test Report|第\\s*[0-9]+\\s*页/共\\s*[0-9]+\\s*页|报告编号：.*|检验检测报告.*|江西中标检检测中心有限公司|CHINA STANDARDTEST)$")

// parseDetailsFromBoxes reconstructs a column-wise table from PaddleOCR boxes. The online OCR
// returns text in reading order that interleaves cells, but each box carries its position, so the
// physical columns (序号/检验项目/技术要求/单位/检验方法/检验结果/单项判定) can be restored.
func parseDetailsFromBoxes(rule *Rule, pages []OCRPage, startPage, endPage int) ([]Detail, []Issue, bool) {
	if startPage <= 0 {
		return nil, nil, false
	}
	headerBox := map[string]OCRBox{}
	for _, p := range pages {
		if p.Page < startPage || p.Page > endPage {
			continue
		}
		for _, b := range p.Boxes {
			for _, h := range detailColumnHeaders {
				if b.Text == h {
					headerBox[h] = b
				}
			}
		}
	}
	if len(headerBox) < 5 {
		return nil, nil, false
	}
	centers := make([]float64, 0, len(detailColumnHeaders))
	for _, h := range detailColumnHeaders {
		b := headerBox[h]
		centers = append(centers, b.X+b.W/2)
	}
	bounds := make([]float64, 0, len(centers)-1)
	for i := 0; i+1 < len(centers); i++ {
		bounds = append(bounds, (centers[i]+centers[i+1])/2)
	}
	colOf := func(x float64) int {
		for i, b := range bounds {
			if x < b {
				return i
			}
		}
		return len(detailColumnHeaders) - 1
	}

	type pageBox struct {
		OCRBox
		page int
	}
	var all []pageBox
	headerY := -1.0
	for _, b := range headerBox {
		if headerY < 0 || b.Y < headerY {
			headerY = b.Y
		}
	}
	for _, p := range pages {
		if p.Page < startPage || p.Page > endPage {
			continue
		}
		for _, b := range p.Boxes {
			if strings.TrimSpace(b.Text) == "" || detailNoiseRE.MatchString(strings.TrimSpace(b.Text)) {
				continue
			}
			if p.Page == startPage && b.Y-headerY < 40 {
				skipHeader := false
				for _, h := range detailColumnHeaders {
					if b.Text == h {
						skipHeader = true
						break
					}
				}
				if skipHeader {
					continue
				}
			}
			all = append(all, pageBox{OCRBox: b, page: p.Page})
		}
	}
	if len(all) == 0 {
		return nil, nil, false
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].page != all[j].page {
			return all[i].page < all[j].page
		}
		cyi := all[i].Y + all[i].H/2
		cyj := all[j].Y + all[j].H/2
		if cyi != cyj {
			return cyi < cyj
		}
		return all[i].X < all[j].X
	})

	var details []Detail
	var issues []Issue
	var buffer []pageBox
	bufferHasNumber := false
	bufferHasJudgement := false
	currentPage := -1

	flush := func(force bool) {
		defer func() { buffer, bufferHasNumber, bufferHasJudgement = nil, false, false }()
		if len(buffer) == 0 {
			return
		}
		if !force && !bufferHasNumber && !bufferHasJudgement {
			buffer = nil
			return
		}
		textsByCol := make([][]string, len(detailColumnHeaders))
		allTexts := make([]string, 0, len(buffer))
		page := buffer[0].page
		for _, b := range buffer {
			col := colOf(b.X + b.W/2)
			if col < 0 || col >= len(textsByCol) {
				continue
			}
			textsByCol[col] = append(textsByCol[col], b.Text)
			allTexts = append(allTexts, b.Text)
		}
		itemTexts := make([]string, 0, len(textsByCol[1]))
		for _, t := range textsByCol[1] {
			if detailRowNumberRE.MatchString(t) || detailWatermarkRE.MatchString(t) {
				continue
			}
			itemTexts = append(itemTexts, t)
		}
		d := Detail{
			ItemName:    strings.Join(itemTexts, "、"),
			Requirement: strings.Join(textsByCol[2], " "),
			Unit:        strings.Join(textsByCol[3], " "),
			Method:      strings.Join(textsByCol[4], " "),
			ResultRaw:   strings.Join(textsByCol[5], " "),
			Judgement:   strings.Join(textsByCol[6], " "),
			Position:    len(details),
			SourcePage:  page,
			SourceText:  strings.Join(allTexts, " "),
		}
		if d.ItemName == "" && d.Requirement == "" && d.ResultRaw == "" && d.Judgement == "" {
			return
		}
		details = append(details, d)
	}

	for _, b := range all {
		page := b.page
		if currentPage >= 0 && page != currentPage {
			flush(false)
		}
		currentPage = page
		col := colOf(b.X + b.W/2)
		if col == 0 && detailRowNumberRE.MatchString(b.Text) {
			buffer = append(buffer, b)
			bufferHasNumber = true
			flush(true)
			continue
		}
		if col == len(detailColumnHeaders)-1 && (strings.Contains(b.Text, "合格") || strings.Contains(b.Text, "不合格")) {
			bufferHasJudgement = true
		}
		buffer = append(buffer, b)
		if len(buffer) > 300 {
			flush(true)
		}
	}
	flush(false)
	if len(details) == 0 {
		return nil, nil, false
	}
	// Only accept a coordinate reconstruction when the rows are clean. A jumbled table must fall
	// back to the line parser (and then to review) rather than store invented detail rows.
	bad := 0
	for _, d := range details {
		if d.ItemName == "" || d.Requirement == "" || d.Judgement == "" || detailWatermarkRE.MatchString(d.ItemName) || strings.Contains(d.SourceText, "检验项目") || strings.Contains(d.SourceText, "技术要求") {
			bad++
		}
	}
	if bad*2 > len(details) {
		return nil, nil, false
	}
	return details, issues, true
}

func assignField(res *ParseResult, name, value string, line docLine) {
	res.Fields[name] = value
	res.RawFields[name] = strings.TrimSpace(line.text)
	res.FieldSources[name] = FieldSource{Raw: strings.TrimSpace(line.text), Page: line.page, Line: line.no}
}

// labelSuffixValue rejects the classic "型号规格" trap: without a colon the regex can capture
// the tail of the label itself ("规格") and store it as the value.
func labelSuffixValue(fr FieldRule, value string) bool {
	for _, label := range fr.Labels {
		if value == label {
			return true
		}
		if value != "" && strings.Contains(label, value) {
			return true
		}
	}
	return false
}

func coerceFieldValue(name string, fr FieldRule, value string, c *compiledRule) (string, bool) {
	value = cleanFieldValue(value)
	if value == "" {
		return "", false
	}
	if fr.Format == "date" {
		d, err := normalizeDate(value)
		if err != nil {
			return "", false
		}
		return d, true
	}
	if re, ok := c.fieldFormats[name]; ok && !re.MatchString(value) {
		return "", false
	}
	return value, true
}

// adjacentFieldValue looks at up to three following and two preceding non-empty lines of the
// same page. It is intentionally conservative: format checks, label lines, watermark noise and
// values already assigned to another field are all rejected.
func adjacentFieldValue(name string, fr FieldRule, re *regexp.Regexp, c *compiledRule, lines []docLine, idx int, allLabels []string, taken map[string]string) (string, docLine, bool) {
	try := func(j int) (string, docLine, bool, bool) {
		if j < 0 || j >= len(lines) || lines[j].page != lines[idx].page {
			return "", docLine{}, false, false
		}
		text := strings.TrimSpace(lines[j].text)
		if text == "" || strings.HasSuffix(text, ":") || strings.HasSuffix(text, "：") {
			return "", docLine{}, false, false
		}
		if labelLine(text, allLabels) {
			// An optional field must not jump over the next label and steal its value.
			return "", docLine{}, false, !fr.Required
		}
		if duplicateFieldValue(text, taken) {
			return "", docLine{}, false, false
		}
		if fr.Format == "date" {
			candidate := text
			if m := re.FindStringSubmatch(text); m != nil {
				candidate = matchValue(m)
			}
			d, err := normalizeDate(candidate)
			if err != nil {
				return "", docLine{}, false, false
			}
			return d, lines[j], true, false
		}
		if !plausibleAdjacentValue(text) {
			return "", docLine{}, false, false
		}
		if reFmt, ok := c.fieldFormats[name]; ok && !reFmt.MatchString(text) {
			return "", docLine{}, false, false
		}
		return cleanFieldValue(text), lines[j], true, false
	}
	maxNext, maxPrev := 3, 2
	if !fr.Required {
		// Optional fields get one adjacent line only; a wider window would let a table header
		// ("样品等级", "商标") become a field value.
		maxNext, maxPrev = 1, 1
	}
	for d := 1; d <= maxNext; d++ {
		if value, line, ok, stop := try(idx + d); ok {
			return value, line, true
		} else if stop {
			break
		}
	}
	for d := 1; d <= maxPrev; d++ {
		if value, line, ok, stop := try(idx - d); ok {
			return value, line, true
		} else if stop {
			break
		}
	}
	return "", docLine{}, false
}

func labelLine(text string, labels []string) bool {
	for _, label := range labels {
		if text == label || strings.HasPrefix(text, label+":") || strings.HasPrefix(text, label+"：") {
			return true
		}
	}
	return false
}

func duplicateFieldValue(text string, taken map[string]string) bool {
	text = strings.TrimSpace(text)
	for _, value := range taken {
		if text == strings.TrimSpace(value) {
			return true
		}
	}
	return false
}

var standardValueRE = regexp.MustCompile("(?i)^(?:GB|SN|QB|HJ|ISO|ASTM|EN|JC)(?:/|\\s)")

var adjacentStopValues = map[string]bool{
	"样品等级": true, "商标": true, "样品编号": true, "抽样基数": true, "样品数量": true,
	"样品特性": true, "检验类别": true, "检测类别": true, "型号规格": true, "规格型号": true,
	"生产单位": true, "受检单位": true, "委托单位": true, "检验项目": true, "技术要求": true,
	"检验结果": true, "单项判定": true, "检验方法": true, "样品名称": true,
}

func plausibleAdjacentValue(text string) bool {
	if len([]rune(text)) < 2 {
		return false
	}
	if adjacentStopValues[strings.TrimSpace(text)] {
		return false
	}
	if standardValueRE.MatchString(text) {
		return true
	}
	hasHan := false
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			hasHan = true
			break
		}
	}
	if !hasHan {
		return false
	}
	upper := strings.ToUpper(text)
	for _, noise := range []string{"CERTIFICATION", "INSPECTION", "JIANG", "XQI", "TEST REPORT", "SAMPLE", "TEST KIND", "CLIENT", "MODEL", "GRADE", "BRAND", "ADDRESS", "SAMPLING", "ENTERPRISE", "CHINA STANDARD", "MANUFACTURE", "SUPERVISE"} {
		if strings.Contains(upper, noise) {
			return false
		}
	}
	return true
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

func startsWithAny(s string, needles []string) bool {
	for _, n := range needles {
		if n != "" && strings.HasPrefix(s, n) {
			return true
		}
	}
	return false
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
