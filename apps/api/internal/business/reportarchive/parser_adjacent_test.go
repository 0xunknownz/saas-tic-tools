package reportarchive

import "testing"

// Real layouts often print the label and its value on separate lines; the parser must use a
// bounded adjacent lookup without stealing a value that belongs to another field.
func TestParseDocumentAdjacentLabels(t *testing.T) {
	rule := seedRule(t)
	rule.Config.Fields["category"] = FieldRule{Labels: []string{"检验类别", "检测类别"}, Pattern: "(?:检验|检测)类别\\s*[:：]?\\s*([^\\n]+)"}
	rule.Config.Fields["model"] = FieldRule{Labels: []string{"型号", "型号规格"}, Pattern: "(?:型号|规格型号|型号规格)\\s*[:：]?\\s*([^\\n]+)"}
	rule.Config.Fields["reportDate"] = FieldRule{Labels: []string{"报告日期", "样品完检/报告日期"}, Pattern: "报告日期\\s*[:：]?\\s*([0-9]{4}\\s*[-年/.]\\s*[0-9]{1,2}\\s*[-月/.]\\s*[0-9]{1,2}\\s*日?)", Required: true, Format: "date"}
	text := `江西中标检检测中心有限公司
报告编号：ZBJ-260725-114
产品名称
万向脚轮
检验类别
抽样检验
委托单位
广东鸿业家具集团有限公司
生产单位：
型号规格
广东鸿业家具集团有限公司
样品完检/报告日期
包装完好
Sample Test
2026年09月29日
经检验，所检项目合格。
检验结论
检验检测专用章：
报告日期：2026年0份验检测专用章
`
	res := parseDocument(&rule, []OCRPage{{Page: 1, Text: text}})
	if res.Fields["productName"] != "万向脚轮" {
		t.Fatalf("productName = %q; issues=%+v", res.Fields["productName"], res.Issues)
	}
	if res.Fields["category"] != "抽样检验" {
		t.Fatalf("category = %q", res.Fields["category"])
	}
	if res.Fields["clientUnit"] != "广东鸿业家具集团有限公司" {
		t.Fatalf("clientUnit = %q", res.Fields["clientUnit"])
	}
	if res.Fields["reportDate"] != "2026-09-29" {
		t.Fatalf("reportDate = %q", res.Fields["reportDate"])
	}
	if res.Fields["conclusion"] != "经检验，所检项目合格。" {
		t.Fatalf("conclusion = %q", res.Fields["conclusion"])
	}
	if res.Fields["institution"] != "江西中标检检测中心有限公司" {
		t.Fatalf("institution = %q", res.Fields["institution"])
	}
	if res.Fields["model"] != "" {
		t.Fatalf("model should stay empty, got %q", res.Fields["model"])
	}
	for _, issue := range res.Issues {
		if issue.Code == "FIELD_MISSING" {
			t.Fatalf("unexpected missing field: %+v", issue)
		}
	}
}
