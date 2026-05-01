package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
)

func parseTimeRange(c *gin.Context, defaultDuration time.Duration) (from, to time.Time) {
	to = time.Now().UTC()
	from = to.Add(-defaultDuration)

	if s := c.Query("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			from = t
		}
	}
	if s := c.Query("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			to = t
		}
	}
	return from, to
}
