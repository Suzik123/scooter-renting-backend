package stripeclient

import (
	"context"
	"fmt"

	stripe "github.com/stripe/stripe-go/v82"
)

// ListPaymentMethods returns the customer's card payment methods. The
// is_default flag is computed against the customer's invoice_settings
// default_payment_method.
func (c *Client) ListPaymentMethods(ctx context.Context, customerID string) ([]PaymentMethodView, error) {
	defaultID, err := c.GetCustomerDefaultPaymentMethod(ctx, customerID)
	if err != nil {
		return nil, err
	}

	params := &stripe.PaymentMethodListParams{
		Customer: stringPtr(customerID),
		Type:     stringPtr("card"),
	}

	out := []PaymentMethodView{}
	for pm, err := range c.sc.V1PaymentMethods.List(ctx, params) {
		if err != nil {
			return nil, fmt.Errorf("stripe.ListPaymentMethods: %w", err)
		}
		if pm == nil {
			continue
		}
		view := PaymentMethodView{ID: pm.ID}
		if pm.Card != nil {
			view.Brand = asString(pm.Card.Brand)
			view.Last4 = pm.Card.Last4
			view.ExpMonth = int(pm.Card.ExpMonth)
			view.ExpYear = int(pm.Card.ExpYear)
		}
		view.IsDefault = defaultID != "" && defaultID == pm.ID
		out = append(out, view)
	}
	return out, nil
}

// DetachPaymentMethod removes a payment method from the customer it is
// attached to.
func (c *Client) DetachPaymentMethod(ctx context.Context, paymentMethodID string) error {
	if _, err := c.sc.V1PaymentMethods.Detach(ctx, paymentMethodID, &stripe.PaymentMethodDetachParams{}); err != nil {
		return fmt.Errorf("stripe.DetachPaymentMethod: %w", err)
	}
	return nil
}
