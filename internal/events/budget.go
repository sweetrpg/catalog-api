package events

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type budgetKey struct{}

// publishSpent is the time one request has spent waiting on the broker, in nanoseconds.
type publishSpent struct{ atomic.Int64 }

// PublishBudget makes all of a request's publishes share one publish timeout, so a bulk write
// can't be held for a timeout per event while the broker is down. Once the budget is spent,
// the request's remaining events are dropped and logged. Only publish time counts, not the
// request's other work.
func PublishBudget() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), budgetKey{}, &publishSpent{}))
		c.Next()
	}
}

// publishAllowance returns how long the next publish may take and a function to call when it
// finishes. Without a request budget, every publish gets the full timeout.
func publishAllowance(ctx context.Context, timeout time.Duration) (time.Duration, func()) {
	spent, ok := ctx.Value(budgetKey{}).(*publishSpent)
	if !ok {
		return timeout, func() {}
	}
	start := time.Now()
	return timeout - time.Duration(spent.Load()), func() { spent.Add(int64(time.Since(start))) }
}
