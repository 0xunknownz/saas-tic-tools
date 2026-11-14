package reportarchive

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func seedRule(t *testing.T) Rule {
	t.Helper()
	cfg := RuleConfig{
		TemplateKey:   "tic-report-v1",
		Name:          "江西中标检检测报告（首套版式）",
		Institution:   "江西中标检检测中心有限公司",
		LayoutMarkers: []string{"检测报告", "报告编号", "检验结论", "检测项目"},
		Fields: map[string]FieldRule{
			"reportNumber": {Labels: []string{"报告编号"}, Pattern: "报告编号\\s*[:：]?\\s*([A-Za-z0-9-]{6,40})", Required: true, Format: "^[A-Z]{2,6}-[0-9]{6}-[0-9]{3}$", Exclude: []string{"分包"}},
			"productName":  {Labels: []string{"产品名称"}, Pattern: "产品名称\\s*[:：]?\\s*([^\\n]+)", Required: true},
			"institution":  {Labels: []string{"检测机构"}, Pattern: "检测机构\\s*[:：]?\\s*([^\\n]+)", Required: true, Exclude: []string{"分包"}},
			"clientUnit":   {Labels: []string{"委托单位"}, Pattern: "委托单位\\s*[:：]?\\s*([^\\n]+)", Required: true},
			"reportDate":   {Labels: []string{"报告日期"}, Pattern: "报告日期\\s*[:：]?\\s*([0-9]{4}\\s*[-年/.]\\s*[0-9]{1,2}\\s*[-月/.]\\s*[0-9]{1,2}\\s*日?)", Required: true, Format: "date", Exclude: []string{"抽样", "送检", "上传", "创建", "打印"}},
			"conclusion":   {Labels: []string{"检验结论", "检测结论"}, Pattern: "(?:检验|检测)结论\\s*[:：]?\\s*([^\\n]+)", Required: true},
		},
		Details: DetailRule{
			StartMarkers:   []string{"检测项目"},
			EndMarkers:     []string{"备注", "以下空白"},
			SkipPatterns:   []string{"序号", "技术要求\\s+单位", "第\\s*[0-9]+\\s*页", "共\\s*[0-9]+\\s*页"},
			SectionPattern: "^(?P<section>[一二三四五六七八九十]+、[^\\s].*)$",
			RowPattern:     "^(?:[0-9]+[.、]?\\s*)?(?P<item>.+?)\\s+(?P<requirement>[≤≥<>＜＞]?\\s*[0-9]+(?:\\.[0-9]+)?(?:\\s*[~～-]\\s*[0-9.]+)?|不得检出|/)\\s*(?P<unit>%|mg/kg|g/kg|mg/L|g/L|mm|MPa|N|℃|h|min|/)?\\s*(?P<method>(?:GB|HJ|ISO|ASTM|EN|QB|JC)[^\\s]*\\s*[0-9-]*)?\\s*(?P<result>未检出(?:[（(][^）)]*[）)])?|[0-9]+(?:\\.[0-9]+)?)\\s*(?P<judgement>合格|不合格|/)$",
			MinRows:        1,
			ExpectedRows:   34,
			RequireFields:  []string{"item", "requirement", "result", "judgement"},
		},
		Validation: ValidationRule{RequiredFields: []string{"reportNumber", "productName", "institution", "clientUnit", "reportDate", "conclusion"}, RequireDetails: true},
	}
	if err := validateRuleConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return Rule{ID: "rule-1", TenantID: "tenant-1", TemplateKey: cfg.TemplateKey, Name: cfg.Name, Version: 1, Status: "active", Config: cfg}
}

func plasticizerNames() []string {
	return []string{
		"邻苯二甲酸二(2-乙基己基)酯（DEHP）", "邻苯二甲酸二丁酯（DBP）", "邻苯二甲酸丁苄酯（BBP）", "邻苯二甲酸二异壬酯（DINP）", "邻苯二甲酸二异癸酯（DIDP）",
		"邻苯二甲酸二辛酯（DOP）", "邻苯二甲酸二甲酯（DMP）", "邻苯二甲酸二乙酯（DEP）", "邻苯二甲酸二异丁酯（DIBP）", "邻苯二甲酸二丙酯（DPP）",
		"邻苯二甲酸二苯酯（DPhP）", "邻苯二甲酸二环己酯（DCHP）", "邻苯二甲酸二壬酯（DNP）", "邻苯二甲酸二戊酯（DNPP）", "邻苯二甲酸二正己酯（DNHP）",
		"邻苯二甲酸二正辛酯（DNOP）", "邻苯二甲酸二甲氧基乙酯（DMEP）", "邻苯二甲酸二乙氧基乙酯（DEEP）", "邻苯二甲酸二丙氧基乙酯（DPPG）", "邻苯二甲酸二丁氧基乙酯（DBEG）",
		"邻苯二甲酸二苯氧基乙酯（DPhEG）", "邻苯二甲酸二(4-甲基-2-戊基)酯（DMPP）", "邻苯二甲酸二异辛酯（DIOP）", "邻苯二甲酸二(2-丙氧基乙基)酯（DPOEG）", "邻苯二甲酸二(2-乙氧基乙基)酯（DEEOEG）",
		"邻苯二甲酸二异十三酯（DITP）", "邻苯二甲酸二(2-丁氧基乙基)酯（DBOEG）", "邻苯二甲酸二(2-甲氧基乙基)酯（DMOEG）", "邻苯二甲酸二乙己酯（DEHA）", "邻苯二甲酸二环辛酯（DCP）",
		"邻苯二甲酸二(2-甲氧基乙氧基乙基)酯（DMEOEG）", "邻苯二甲酸二(2-乙氧基乙氧基乙基)酯（DEEOEG2）", "邻苯二甲酸二(2-乙基己基)酯（DEHP）", "邻苯二甲酸二(2-乙基己基)酯（DEHP）",
	}
}

func testPages() []OCRPage {
	page1 := `江西中标检检测中心有限公司
检测报告
报告编号：ZBJ-260725-114
产品名称：万向脚轮
检测机构：江西中标检检测中心有限公司
委托单位：广东鸿业家具集团有限公司
受检单位：广东鸿业家具集团有限公司
生产单位：广东鸿业家具集团有限公司
型号：/
检验类别：型式检验
检验依据：GB/T 3324-2017
报告日期：2026-09-29
检验结论：经检验，所检项目合格
备注：/`
	page2 := `万向脚轮检测报告
第 1 页 共 3 页
检测项目 技术要求 单位 检验方法 检测结果 单项判定
一、增塑剂`
	page3 := `万向脚轮检测报告
第 2 页 共 3 页
检测项目 技术要求 单位 检验方法 检测结果 单项判定`
	names := plasticizerNames()
	for i := 0; i < 17; i++ {
		page2 += fmt.Sprintf("\n%d %s ≤0.1 %% GB/T 22048-2015 未检出 合格", i+1, names[i])
	}
	for i := 17; i < 34; i++ {
		page3 += fmt.Sprintf("\n%d %s ≤0.1 %% GB/T 22048-2015 未检出 合格", i+1, names[i])
	}
	page4 := `万向脚轮检测报告
第 3 页 共 3 页
备注：/`
	return []OCRPage{{Page: 1, Text: page1}, {Page: 2, Text: page2}, {Page: 3, Text: page3}, {Page: 4, Text: page4}}
}

func TestParseDocumentExtractsFieldsAndCrossPageDetails(t *testing.T) {
	rule := seedRule(t)
	res := parseDocument(&rule, testPages())
	for _, field := range []string{"reportNumber", "productName", "institution", "clientUnit", "reportDate", "conclusion"} {
		if strings.TrimSpace(res.Fields[field]) == "" {
			t.Fatalf("missing %s: issues=%+v", field, res.Issues)
		}
	}
	if res.Fields["reportNumber"] != "ZBJ-260725-114" {
		t.Fatalf("report number = %q", res.Fields["reportNumber"])
	}
	if res.Fields["reportDate"] != "2026-09-29" {
		t.Fatalf("report date = %q", res.Fields["reportDate"])
	}
	if res.Fields["reportName"] != "万向脚轮检测报告" {
		t.Fatalf("report name = %q", res.Fields["reportName"])
	}
	if len(res.Details) != 34 {
		t.Fatalf("details = %d, want 34; issues=%+v", len(res.Details), res.Issues)
	}
	for i, d := range res.Details {
		if d.ItemName == "" || d.ResultRaw == "" || d.Judgement != "合格" {
			t.Fatalf("detail %d incomplete: %+v", i+1, d)
		}
		if d.Section != "一、增塑剂" {
			t.Fatalf("detail %d section = %q", i+1, d.Section)
		}
	}
	if res.Details[33].SourcePage != 3 || res.Details[0].SourcePage != 2 {
		t.Fatalf("source pages: first=%d last=%d", res.Details[0].SourcePage, res.Details[33].SourcePage)
	}
	for _, issue := range res.Issues {
		t.Fatalf("unexpected issue: %+v", issue)
	}
}

func TestParseDocumentKeepsNotDetectedAndUnparsedLines(t *testing.T) {
	rule := seedRule(t)
	pages := testPages()
	pages[1].Text = strings.Replace(pages[1].Text, "1 邻苯二甲酸二(2-乙基己基)酯（DEHP） ≤0.1 % GB/T 22048-2015 未检出 合格", "1 邻苯二甲酸二(2-乙基己基)酯（DEHP） ≤0.1 % GB/T 22048-2015 未检出（<0.01） 合格", 1)
	res := parseDocument(&rule, pages)
	if len(res.Details) != 34 {
		t.Fatalf("details = %d", len(res.Details))
	}
	if !strings.Contains(res.Details[0].ResultRaw, "未检出（<0.01）") {
		t.Fatalf("result = %q", res.Details[0].ResultRaw)
	}
}

func TestParseDocumentFlagsUnparsedDetailRows(t *testing.T) {
	rule := seedRule(t)
	pages := testPages()
	pages[1].Text = strings.Replace(pages[1].Text, "2 邻苯二甲酸二丁酯（DBP） ≤0.1 % GB/T 22048-2015 未检出 合格", "2 邻苯二甲酸二丁酯（DBP） 坏列 未检出 合格", 1)
	res := parseDocument(&rule, pages)
	found := false
	for _, issue := range res.Issues {
		if issue.Code == "UNPARSED_DETAIL_LINE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unparsed detail issue, got %+v", res.Issues)
	}
}

func TestChooseRulePrefersTenantAndFlagsAmbiguity(t *testing.T) {
	pages := testPages()
	text := ocrPageText(pages)
	builtin := seedRule(t)
	builtin.ID = "builtin"
	builtin.Builtin = true
	tenant := seedRule(t)
	tenant.ID = "tenant"
	tenant.Builtin = false

	got, issues := chooseRule([]Rule{builtin, tenant}, text)
	if got == nil || got.ID != "tenant" || len(issues) != 0 {
		t.Fatalf("got=%+v issues=%+v", got, issues)
	}
	tenant2 := seedRule(t)
	tenant2.ID = "tenant2"
	got, issues = chooseRule([]Rule{builtin, tenant, tenant2}, text)
	if got != nil || len(issues) != 1 || issues[0].Code != "RULE_AMBIGUOUS" {
		t.Fatalf("got=%+v issues=%+v", got, issues)
	}
}

func TestValidateRuleConfigRejectsUnsafeAndBrokenPatterns(t *testing.T) {
	rule := seedRule(t)
	cfg := rule.Config
	cfg.Fields["bad"] = FieldRule{Labels: []string{"bad"}, Pattern: "(unclosed"}
	if err := validateRuleConfig(cfg); err == nil {
		t.Fatal("bad pattern accepted")
	}
	cfg = rule.Config
	cfg.Details.RequireFields = []string{"script"}
	if err := validateRuleConfig(cfg); err == nil {
		t.Fatal("unsupported required field accepted")
	}
}

func TestNormalizeDateRejectsInvalid(t *testing.T) {
	if got, err := normalizeDate("2026年9月29日"); err != nil || got != "2026-09-29" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if _, err := normalizeDate("抽样日期：2026-13-40"); err == nil {
		t.Fatal("invalid date accepted")
	}
	_ = time.Now
}

func TestParseDocumentUsesReportDateNotSamplingDate(t *testing.T) {
	rule := seedRule(t)
	pages := testPages()
	pages[0].Text = strings.Replace(pages[0].Text, "报告日期：2026-09-29", "抽样日期：2026-09-01\n报告日期：2026-09-29", 1)
	res := parseDocument(&rule, pages)
	if res.Fields["reportDate"] != "2026-09-29" {
		t.Fatalf("report date = %q", res.Fields["reportDate"])
	}
}

func TestParseDocumentKeepsMultipleTechnicalRequirements(t *testing.T) {
	rule := seedRule(t)
	pages := testPages()
	pages[1].Text = strings.Replace(pages[1].Text,
		"1 邻苯二甲酸二(2-乙基己基)酯（DEHP） ≤0.1 % GB/T 22048-2015 未检出 合格",
		"1 邻苯二甲酸二(2-乙基己基)酯（DEHP） ≤0.1 % GB/T 22048-2015 未检出 合格\n1 邻苯二甲酸二(2-乙基己基)酯（DEHP） ≤0.05 % GB/T 22048-2015 未检出 合格", 1)
	res := parseDocument(&rule, pages)
	var sameItem []Detail
	for _, detail := range res.Details {
		if detail.ItemName == "邻苯二甲酸二(2-乙基己基)酯（DEHP）" && detail.SourcePage == 2 && detail.Position < 2 {
			sameItem = append(sameItem, detail)
		}
	}
	if len(sameItem) != 2 || sameItem[0].Requirement != "≤0.1" || sameItem[1].Requirement != "≤0.05" {
		t.Fatalf("multi requirement rows = %+v", sameItem)
	}
}

func TestDeriveDetectionKeepsUnqualifiedArchiveable(t *testing.T) {
	if got := deriveDetection("经检验，所检项目不合格"); got != "unqualified" {
		t.Fatalf("unqualified = %q", got)
	}
	if got := deriveDetection("经检验，所检项目合格"); got != "qualified" {
		t.Fatalf("qualified = %q", got)
	}
	if got := deriveDetection(""); got != "unknown" {
		t.Fatalf("unknown = %q", got)
	}
}

func TestParseDocumentSecondLayoutVariant(t *testing.T) {
	rule := seedRule(t)
	variant := []OCRPage{
		{Page: 1, Text: "江西中标检检测中心有限公司\n检 测 报 告\n产品名称：工业脚轮\n报告编号：ZBJ-261001-208\n检测机构：江西中标检检测中心有限公司\n委托单位：示例制造有限公司\n受检单位：示例制造有限公司\n报告日期：2026-10-08\n检验结论：经检验，所检项目合格\n抽样日期：2026-09-20"},
		{Page: 2, Text: "工业脚轮检测报告\n第 1 页 共 2 页\n检测项目 技术要求 单位 检验方法 检测结果 单项判定\n一、增塑剂\n1 邻苯二甲酸二丁酯（DBP） ≤0.1 % GB/T 22048-2015 未检出 合格\n2 邻苯二甲酸二异壬酯（DINP） ≤0.1 % GB/T 22048-2015 0.05 合格"},
	}
	res := parseDocument(&rule, variant)
	if res.Fields["reportNumber"] != "ZBJ-261001-208" || res.Fields["productName"] != "工业脚轮" || res.Fields["reportDate"] != "2026-10-08" {
		t.Fatalf("variant fields = %+v", res.Fields)
	}
	if len(res.Details) != 2 || res.Details[1].ResultRaw != "0.05" {
		t.Fatalf("variant details = %+v", res.Details)
	}
}
