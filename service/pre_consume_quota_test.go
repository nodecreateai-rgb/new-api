package service

import (
	"os"
	"strings"
	"testing"
)

func TestInsufficientQuotaPublicMessageHidesCost(t *testing.T) {
	err := insufficientUserQuotaAPIError()
	if err == nil {
		t.Fatal("expected insufficient quota error")
	}
	msg := err.Error()
	if msg != "用户额度不足" {
		t.Fatalf("public message = %q, want 用户额度不足", msg)
	}
	for _, forbidden := range []string{"需要预扣费额度", "剩余额度", "¥", "预扣费额度失败"} {
		if strings.Contains(msg, forbidden) {
			t.Fatalf("public message leaked %q: %q", forbidden, msg)
		}
	}

	for _, name := range []string{"pre_consume_quota.go", "billing_session.go"} {
		data, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		body := string(data)
		if strings.Count(body, `fmt.Errorf("用户额度不足, 剩余额度:`) > 0 {
			t.Fatalf("%s still returns remaining quota to the client", name)
		}
		if strings.Count(body, `fmt.Errorf("预扣费额度失败, 用户剩余额度:`) > 0 {
			t.Fatalf("%s still returns pre-consume cost to the client", name)
		}
	}
}
