package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
)

func TestRestrictedGroupsOnlyForAssignedUser(t *testing.T) {
	_ = setting.UpdateUserUsableGroupsByJSONString(`{"default":"d","vip11":"V11","vip9":"V9"}`)
	if _, ok := GetUserUsableGroups("default")["vip11"]; ok {
		t.Fatal("default user must not see vip11")
	}
	g := GetUserUsableGroups("vip11")
	if _, ok := g["vip11"]; !ok {
		t.Fatal("vip11 user must keep vip11")
	}
	if _, ok := g["vip9"]; ok {
		t.Fatal("vip11 user must not get vip9")
	}
	if _, ok := g["default"]; !ok {
		t.Fatal("default stays public")
	}
}
