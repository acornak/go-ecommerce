package cards

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stripe/stripe-go/v73"
)

func Test_CardErrorMessage(t *testing.T) {
	// table-driven testing
	testCases := map[stripe.ErrorCode]string{
		stripe.ErrorCodeCardDeclined:        "Your card has been declined",
		stripe.ErrorCodeExpiredCard:         "Your card is expired",
		stripe.ErrorCodeIncorrectCVC:        "Incorrect CVC code",
		stripe.ErrorCodeIncorrectZip:        "Incorrect ZIP/postal code",
		stripe.ErrorCodeAmountTooLarge:      "The amount is too large to charge to your card",
		stripe.ErrorCodeAmountTooSmall:      "The amount is too small to charge to your card",
		stripe.ErrorCodeBalanceInsufficient: "Insufficient balance",
		stripe.ErrorCodePostalCodeInvalid:   "Incorrect postal code",
		stripe.ErrorCodeAPIKeyExpired:       "Your card has been declined",
	}

	for k, v := range testCases {
		assert.Equal(t, cardErrorMessage(k), v)
	}
}
