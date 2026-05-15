// Package messaging wraps github.com/rabbitmq/amqp091-go with the topology
// and lifecycle helpers used by the UniScoot api + worker binaries.
package messaging

// Topic exchanges declared by the api and consumed by the worker.
const (
	ExchangeNotifications = "notifications.events"
	ExchangePayments      = "payments.events"
)

// Routing keys for payments + rentals + notifications.
const (
	RKPaymentSucceeded        = "payment.succeeded"
	RKPaymentFailed           = "payment.failed"
	RKPaymentOfflineApproved  = "payment.offline_approved"
	RKRentalStarted           = "rental.started"
	RKRentalCompleted         = "rental.completed"
	RKNotificationEmailSend   = "notification.email.send"
)
