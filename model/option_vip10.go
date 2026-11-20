package model

import "math"

const (
	vip10GroupName         = "vip10"
	vip10PriceOff          = 0.1
	vip10Fast480Model      = "seedance-2.0-fast-480p"
	vip10Fast480Price      = 1.4
	vip10MinimaxH3768Model = "minimax-h3-768p"
	vip10MinimaxH3768Price = 0.5
)

// vip10DedicatedPrices skip the automatic sticker-¥0.1 cut. seedance-2.0-fast-480p
// stays at ¥1.4 (already discounted). minimax-h3-768p keeps sticker ¥0.5 (no vip10 discount).
var vip10DedicatedPrices = map[string]float64{
	vip10Fast480Model:      vip10Fast480Price,
	vip10MinimaxH3768Model: vip10MinimaxH3768Price,
}

func roundYuan(v float64) float64 {
	return math.Round(v*100) / 100
}

func vip10DiscountedFromBase(base float64) float64 {
	if base <= 0 {
		return base
	}
	want := roundYuan(base - vip10PriceOff)
	if want < 0 {
		return 0
	}
	return want
}

func vip10PriceFor(model string, base float64) float64 {
	if dedicated, ok := vip10DedicatedPrices[model]; ok {
		return dedicated
	}
	return vip10DiscountedFromBase(base)
}

// applyVip10ModelPriceDiscount writes vip10 ModelGroupPrice as sticker-¥0.1.
// Sticker is the default-group price (falling back to ModelPrice). Computing
// from sticker — never from the current vip10 value — keeps a restart from
// stacking another 0.1 off. seedance-2.0-fast-480p stays at ¥1.4 (already
// discounted).
func applyVip10ModelPriceDiscount(modelGroupPrices map[string]map[string]float64, basePrices map[string]float64) bool {
	if modelGroupPrices == nil {
		return false
	}
	vip10 := modelGroupPrices[vip10GroupName]
	if vip10 == nil {
		vip10 = map[string]float64{}
		modelGroupPrices[vip10GroupName] = vip10
	}
	defaultPrices := modelGroupPrices["default"]

	skip := make(map[string]struct{})
	for _, model := range tokenBilledChatModels() {
		skip[model] = struct{}{}
	}
	changed := false
	for model := range skip {
		if _, exists := vip10[model]; exists {
			delete(vip10, model)
			changed = true
		}
	}

	live := make(map[string]struct{})
	for _, groupPrices := range modelGroupPrices {
		for model := range groupPrices {
			live[model] = struct{}{}
		}
	}

	bases := make(map[string]float64)
	if defaultPrices != nil {
		for model, price := range defaultPrices {
			if _, ok := skip[model]; ok {
				continue
			}
			bases[model] = price
		}
	}
	for model, price := range basePrices {
		if _, ok := skip[model]; ok {
			continue
		}
		if _, ok := bases[model]; ok {
			continue
		}
		if _, ok := live[model]; !ok {
			continue
		}
		bases[model] = price
	}

	for model, base := range bases {
		want := vip10PriceFor(model, base)
		if vip10[model] != want {
			vip10[model] = want
			changed = true
		}
	}
	for model, want := range vip10DedicatedPrices {
		if vip10[model] != want {
			vip10[model] = want
			changed = true
		}
	}
	return changed
}
