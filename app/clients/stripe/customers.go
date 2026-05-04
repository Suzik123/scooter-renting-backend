package stripeclient

import (
	"context"
	"fmt"

	stripe "github.com/stripe/stripe-go/v82"
)

// CreateCustomer provisions a new Stripe customer for the given email.
// Returns the customer id (cus_xxx).
func (c *Client) CreateCustomer(ctx context.Context, email, name string) (string, error) {
	params := &stripe.CustomerCreateParams{
		Email: stringPtr(email),
		Name:  stringPtr(name),
	}
	cust, err := c.sc.V1Customers.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("stripe.CreateCustomer: %w", err)
	}
	return cust.ID, nil
}

// GetCustomerDefaultPaymentMethod fetches the customer's invoice_settings
// default_payment_method id, or "" when not set.
func (c *Client) GetCustomerDefaultPaymentMethod(ctx context.Context, customerID string) (string, error) {
	cust, err := c.sc.V1Customers.Retrieve(ctx, customerID, &stripe.CustomerRetrieveParams{})
	if err != nil {
		return "", fmt.Errorf("stripe.GetCustomer: %w", err)
	}
	if cust == nil || cust.InvoiceSettings == nil || cust.InvoiceSettings.DefaultPaymentMethod == nil {
		return "", nil
	}
	return cust.InvoiceSettings.DefaultPaymentMethod.ID, nil
}
