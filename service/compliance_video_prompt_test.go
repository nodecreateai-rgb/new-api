package service

import "strings"
import "testing"

func TestCarryComplianceVideoPromptByMode(t *testing.T) {
	cases := []struct {
		mode   string
		prefix string
	}{
		{"grid", "【最高优先级·首帧前强制去网格】"},
		{"fishnet", "【整脸去网纱】"},
		{"watercolor", "【水彩成片约束】"},
		{"colored-pencil", "【彩铅成片约束】"},
	}
	for _, tc := range cases {
		got := carryComplianceVideoPrompt("角色走进房间", tc.mode)
		if !strings.HasPrefix(got, tc.prefix) || !strings.Contains(got, "【生成视频】角色走进房间") {
			t.Fatalf("mode %s: %s", tc.mode, got)
		}
		again := carryComplianceVideoPrompt(got, tc.mode)
		if strings.Count(again, tc.prefix) != 1 {
			t.Fatalf("mode %s duplicated hint: %s", tc.mode, again)
		}
	}
	face := carryComplianceVideoPrompt("一位女性角色走路", "watercolor")
	if !strings.Contains(face, "双眼全程自然连续") {
		t.Fatalf("portrait watercolor hint missing: %s", face)
	}
	plain := carryComplianceVideoPrompt("产品在桌面上旋转", "watercolor")
	if strings.Contains(plain, "双眼全程自然连续") {
		t.Fatalf("product shot used the portrait hint: %s", plain)
	}
}

func TestComplianceImagePromptMatchesMode(t *testing.T) {
	if !strings.Contains(compliancePrompt("grid"), "square grid") {
		t.Fatal("grid image prompt")
	}
	if !strings.Contains(compliancePrompt("fishnet"), "fishnet") {
		t.Fatal("fishnet image prompt")
	}
	if !strings.Contains(compliancePrompt("watercolor"), "face-local watercolor") {
		t.Fatal("watercolor image prompt")
	}
	if !strings.Contains(compliancePrompt("colored-pencil"), "colored-pencil portrait") {
		t.Fatal("pencil image prompt")
	}
}
