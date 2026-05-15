package events

// RouteFor maps an event type to its (exchange, routing_key) pair. Returns
// empty strings for an unknown type so callers can drop silently.
func RouteFor(typ string) (exchange, routingKey string) {
	switch typ {
	case TypePaymentSucceeded:
		return "payments.events", "payment.succeeded"
	case TypePaymentFailed:
		return "payments.events", "payment.failed"
	case TypeOfflinePaymentApproved:
		return "payments.events", "payment.offline_approved"
	case TypeRentalStarted:
		return "payments.events", "rental.started"
	case TypeRentalCompleted:
		return "payments.events", "rental.completed"
	}
	return "", ""
}
