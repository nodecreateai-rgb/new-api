package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// apiPrefixes are dynamic endpoints (model lists, pricing, tasks) that must
// never be cached, or clients keep showing stale model lists for a week.
var apiPrefixes = []string{"/v1", "/api", "/pg", "/mj", "/suno", "/kling", "/jimeng", "/dashboard"}

func Cache() func(c *gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		switch {
		case path == "/":
			c.Header("Cache-Control", "no-cache")
		case isAPIPath(path):
			c.Header("Cache-Control", "no-store")
		default:
			c.Header("Cache-Control", "max-age=604800") // one week
		}
		c.Header("Cache-Version", "b688f2fb5be447c25e5aa3bd063087a83db32a288bf6a4f35f2d8db310e40b14")
		c.Next()
	}
}

func isAPIPath(path string) bool {
	for _, p := range apiPrefixes {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return strings.HasPrefix(path, "/v1beta")
}
