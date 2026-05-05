package payments

import (
	"errors"
	"fmt"
)

// ============================================================
// THE INTERFACE WAY (lots of plumbing)
// ============================================================

// PaymentService is the interface most Go code reaches for.
type PaymentService interface {
	Charge(userID string, amount int) error
	Refund(txnID string) error
}

// To test code that uses this interface, you need a mock implementation:
//
//   type MockPaymentService struct {
//       ChargeFunc func(userID string, amount int) error
//       RefundFunc func(txnID string) error
//       ChargeCallCount int
//   }
//
//   func (m *MockPaymentService) Charge(userID string, amount int) error {
//       m.ChargeCallCount++
//       return m.ChargeFunc(userID, amount)
//   }
//   func (m *MockPaymentService) Refund(txnID string) error {
//       return m.RefundFunc(txnID)
//   }
//
// 30 lines of plumbing per service. Multiply by every dependency.

// ============================================================
// THE FUNCTIONAL SEAM WAY
// ============================================================

func ProcessOrder(
	userID string,
	amount int,
	charge func(userID string, amount int) error,
	notify func(userID string, msg string) error,
) error {
	if err := charge(userID, amount); err != nil {
		return fmt.Errorf("charge failed: %w", err)
	}
	return notify(userID, "Order confirmed")
}

// ============================================================
// COMPOSITION: BUILD A LIBRARY OF BEHAVIORS
// ============================================================

var ChargeOK = func(uid string, amt int) error {
	return nil
}

var ChargeDeclined = func(uid string, amt int) error {
	return errors.New("declined")
}

var NotifyOK = func(uid, msg string) error {
	return nil
}

var NotifyFails = func(uid, msg string) error {
	return errors.New("email service down")
}

// ============================================================
// TESTS: COMPOSE BEHAVIORS, NO MOCK FRAMEWORK
// ============================================================

func ExampleProcessOrder_happyPath() {
	err := ProcessOrder("alice", 1000, ChargeOK, NotifyOK)
	fmt.Println(err) // Output: <nil>
}

func ExampleProcessOrder_chargeDeclined() {
	err := ProcessOrder("alice", 1000, ChargeDeclined, NotifyOK)
	fmt.Println(err) // Output: charge failed: declined
}

func ExampleProcessOrder_notifyDown() {
	err := ProcessOrder("alice", 1000, ChargeOK, NotifyFails)
	fmt.Println(err) // Output: email service down
}

// ============================================================
// CAPTURE: STUBS THAT RECORD WHAT HAPPENED
// ============================================================

func ExampleProcessOrder_recordsCharge() {
	var charged []int
	chargeRecording := func(uid string, amt int) error {
		charged = append(charged, amt)
		return nil
	}

	ProcessOrder("alice", 1000, chargeRecording, NotifyOK)
	fmt.Println(charged) // Output: [1000]
}

// ============================================================
// THE LESSON
// ============================================================

// Four tests. Three reused behaviors. One inline closure. Zero mock framework.
// The stubs are values you compose. The test is a sentence.
// You have not replaced your mocking library. You made it unnecessary.
