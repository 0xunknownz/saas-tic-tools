package reportarchive

import "testing"

func boxRow(page int, y float64, texts []string) []OCRBox {
	boxes := make([]OCRBox, 0, len(texts))
	for i, t := range texts {
		if t == "" {
			continue
		}
		boxes = append(boxes, OCRBox{Text: t, X: float64(i * 100), Y: y, W: 80, H: 20})
	}
	return boxes
}

// A clean column-wise table restores rows from positions; the line parser alone cannot.
func TestParseDocumentDetailsFromBoxes(t *testing.T) {
	rule := seedRule(t)
	header := []string{"序号", "检验项目", "技术要求", "单位", "检验方法", "检验结果", "单项判定"}
	var boxes []OCRBox
	boxes = append(boxes, boxRow(1, 0, header)...)
	boxes = append(boxes, boxRow(1, 100, []string{"", "外观", "应光洁", "%", "GB/T 4765-2014", "0.1", "合格"})...)
	boxes = append(boxes, boxRow(1, 150, []string{"1"})...)
	boxes = append(boxes, boxRow(1, 200, []string{"", "装配", "应牢固", "N", "GB/T 4765-2014", "5", "合格"})...)
	boxes = append(boxes, boxRow(1, 250, []string{"2"})...)
	pages := []OCRPage{{Page: 1, Text: "检测项目", Boxes: boxes}}
	res := parseDocument(&rule, pages)
	if len(res.Details) != 2 {
		t.Fatalf("details = %d, want 2; issues=%+v", len(res.Details), res.Issues)
	}
	if res.Details[0].ItemName != "外观" || res.Details[0].Requirement != "应光洁" || res.Details[0].ResultRaw != "0.1" || res.Details[0].Judgement != "合格" {
		t.Fatalf("detail 1 = %+v", res.Details[0])
	}
	if res.Details[1].ItemName != "装配" || res.Details[1].SourcePage != 1 {
		t.Fatalf("detail 2 = %+v", res.Details[1])
	}
}
