package events

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type budgetKey struct{}

// publishSpent is the time one request has lost to failed publishes, in nanoseconds.
type publishSpent struct{ atomic.Int64 }

// PublishBudget makes all of a request's failed publishes share one publish timeout, so a bulk
// write can't be held for a timeout per event while the broker is down. Once the budget is
// spent, the request's remaining events are dropped and logged. Successful publishes cost
// nothing, so a large bulk write against a healthy broker drops no events.
func PublishBudget() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), budgetKey{}, &publishSpent{}))
		c.Next()
	}
}

// HasPublishBudget reports whether ctx carries a request publish budget.
func HasPublishBudget(ctx context.Context) bool {
	_, ok := ctx.Value(budgetKey{}).(*publishSpent)
	return ok
}

// publishAllowance returns how long the next publish may take and a function to call with its
// result. Without a request budget, every publish gets the full timeout.
func publishAllowance(ctx context.Context, timeout time.Duration) (time.Duration, func(error)) {
	spent, ok := ctx.Value(budgetKey{}).(*publishSpent)
	if !ok {
		return timeout, func(error) {}
	}
	start := time.Now()
	return timeout - time.Duration(spent.Load()), func(err error) {
		if err != nil {
			spent.Add(int64(time.Since(start)))
		}
	}
}
