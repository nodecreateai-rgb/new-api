package model

import "testing"

func TestSeedance720OnlyVisibleToItsOwner(t *testing.T) {
	if !IsModelVisibleToUser("seedance-2.0-720p", 27) {
		t.Fatal("user 27 (1347314421) must see seedance-2.0-720p")
	}
	for _, uid := range []int{0, 1, 2, 26, 28} {
		if IsModelVisibleToUser("seedance-2.0-720p", uid) {
			t.Fatalf("user %d must not see seedance-2.0-720p", uid)
		}
	}
	if !IsModelVisibleToUser("seedance-2.0-fast-480p", 2) {
		t.Fatal("public models stay visible")
	}
	got := FilterModelsForUser([]string{"a", "seedance-2.0-720p", "b"}, 2)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("filter = %v", got)
	}
	p := FilterPricingForUser([]Pricing{{ModelName: "seedance-2.0-720p"}, {ModelName: "x"}}, 3)
	if len(p) != 1 || p[0].ModelName != "x" {
		t.Fatalf("pricing filter = %v", p)
	}
}
