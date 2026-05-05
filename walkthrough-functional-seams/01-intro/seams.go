package billing

import (
	"fmt"
	"time"
)

// ============================================================
// BEFORE: hardcoded dependency
// ============================================================

// SubscriptionExpired calls time.Now() directly. The caller can't
// control time, can't test edge cases, can't simulate the past.
func SubscriptionExpired(renewedAt time.Time, period time.Duration) bool {
	expiresAt := renewedAt.Add(period)
	return time.Now().After(expiresAt)
}

// ============================================================
// AFTER: a functional seam
// ============================================================

// Same logic. But "what time is it" is a parameter now.
// `now func() time.Time` is the seam. Callers pass any clock.
// No interface. No mock. Just a function.
func SubscriptionExpiredAt(
	renewedAt time.Time,
	period time.Duration,
	now func() time.Time,
) bool {
	expiresAt := renewedAt.Add(period)
	return now().After(expiresAt)
}

// ============================================================
// HOW IT'S USED
// ============================================================

// In production: pass time.Now. The seam disappears at compile time.
func ExpiredInProduction(renewedAt time.Time, period time.Duration) bool {
	return SubscriptionExpiredAt(renewedAt, period, time.Now)
}

// In a test: pass any function returning any time.
// Three lines. The test reads like a fact.
func ExampleExpiredYesterday() {
	yesterday := time.Now().Add(-24 * time.Hour)
	pretendNow := func() time.Time { return time.Now() }
	expired := SubscriptionExpiredAt(yesterday, time.Hour, pretendNow)
	fmt.Println(expired) // Output: true
}

// ============================================================
// WHY THIS BEATS AN INTERFACE
// ============================================================

// The interface version would be:
//
//   type Clock interface { Now() time.Time }
//   func SubscriptionExpiredV2(renewedAt time.Time, period time.Duration, c Clock) bool
//
// You'd need: a Clock type, a real implementation, a mock implementation,
// and test setup that constructs the mock. Three files of plumbing for
// one method.
//
// The functional seam is one parameter. The callsite is the policy.
// You haven't introduced an abstraction — you've made an existing one explicit.
