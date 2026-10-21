package skillfile

import "testing"

func TestValidToolAllowsMcpRef(t *testing.T) {
	if !ValidTool("knowledge_search") || !ValidTool("formula_hint@campus-tools") {
		t.Fatal("expected legal tools")
	}
	for _, bad := range []string{"Formula@campus", "formula-hint@campus", "formula_hint", "formula_hint@"} {
		if ValidTool(bad) {
			t.Fatalf("accepted %s", bad)
		}
	}
	source := "名称：公式提示\n适用条件：分数\n资料：数学\n工具：formula_hint@campus-tools\n启用：是\n\n只提示下一步。\n"
	doc, ok := Parse(source)
	if !ok || doc.Tool != "formula_hint@campus-tools" {
		t.Fatalf("%v %+v", ok, doc)
	}
	commented := "名称：公式提示\n# 这行忽略\n适用条件：分数\n资料：数学\n工具：knowledge_search\n启用：是\n\n只提示下一步。\n"
	if _, ok := Parse(commented); !ok {
		t.Fatal("comment line should be ignored")
	}
	unknown := "名称：公式提示\n备注：多余\n适用条件：分数\n资料：数学\n工具：knowledge_search\n启用：是\n\n只提示下一步。\n"
	if _, ok := Parse(unknown); ok {
		t.Fatal("unknown field should be rejected")
	}
}
