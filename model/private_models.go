package model

// privateModelUsers lists models that only specific users may see or call.
// Everyone else gets "model not found" from the relay and never sees the
// model in /v1/models, /api/user/self/models or /api/pricing.
var privateModelUsers = map[string]map[int]struct{}{
	// seedance-2.0-720p was reserved for user 1347314421 (id 27); now taken
	// down for everyone: an empty allow-list hides it from all users.
	"seedance-2.0-720p": {},
}

// IsModelVisibleToUser reports whether userId may list or call modelName.
func IsModelVisibleToUser(modelName string, userId int) bool {
	allowed, private := privateModelUsers[modelName]
	if !private {
		return true
	}
	_, ok := allowed[userId]
	return ok
}

// FilterModelsForUser drops private models userId may not see.
func FilterModelsForUser(models []string, userId int) []string {
	out := models[:0:0]
	for _, m := range models {
		if IsModelVisibleToUser(m, userId) {
			out = append(out, m)
		}
	}
	return out
}

// FilterPricingForUser drops private models userId may not see.
func FilterPricingForUser(pricing []Pricing, userId int) []Pricing {
	out := make([]Pricing, 0, len(pricing))
	for _, p := range pricing {
		if IsModelVisibleToUser(p.ModelName, userId) {
			out = append(out, p)
		}
	}
	return out
}
