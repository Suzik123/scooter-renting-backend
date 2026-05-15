package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"github.com/uniscoot/scooter-renting-backend/app/internal/config"
)

// Handler is the callback invoked for each message consumed from a queue.
// Returning a non-nil error causes the message to be nack'd with requeue.
type Handler func(ctx context.Context, body []byte, headers amqp.Table) error

// Client owns the AMQP connection and a long-lived publishing channel.
// Reconnects happen in a background goroutine; consumers re-attach automatically.
type Client struct {
	cfg *config.Config
	log *zap.Logger

	mu      sync.RWMutex
	conn    *amqp.Connection
	pubChan *amqp.Channel

	// declared exchanges so reconnect can replay them
	exchanges map[string]string // name -> kind

	// declared consumers so reconnect can replay them
	consumersMu sync.Mutex
	consumers   []consumerSpec

	closeOnce sync.Once
	closed    chan struct{}
}

type consumerSpec struct {
	queue, exchange, routingKey string
	handler                     Handler
}

// NewClient dials Rabbit, opens a long-lived publishing channel, and spawns
// a goroutine that reconnects with exponential backoff on disconnect.
// The initial dial uses the same capped backoff (1s→30s, 60s total budget)
// so docker-compose cold boots tolerate a slow broker without crashing fx.
func NewClient(cfg *config.Config, log *zap.Logger) (*Client, error) {
	if log == nil {
		log = zap.NewNop()
	}
	c := &Client{
		cfg:       cfg,
		log:       log,
		exchanges: make(map[string]string),
		closed:    make(chan struct{}),
	}
	if err := c.dialWithRetry(60 * time.Second); err != nil {
		return nil, err
	}
	go c.watchReconnect()
	return c, nil
}

// dialWithRetry retries c.connect() with capped exponential backoff until
// success or the deadline is hit. Used at startup so api/worker survives the
// usual ~2-second window between RabbitMQ's healthcheck flipping green and
// its AMQP listener accepting connections.
func (c *Client) dialWithRetry(budget time.Duration) error {
	deadline := time.Now().Add(budget)
	backoff := time.Second
	for attempt := 1; ; attempt++ {
		err := c.connect()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("rabbitmq dial budget exhausted after %d attempts: %w", attempt, err)
		}
		c.log.Warn("rabbitmq initial dial failed, retrying",
			zap.Int("attempt", attempt),
			zap.Duration("backoff", backoff),
			zap.Error(err),
		)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}
	}
}

func (c *Client) connect() error {
	conn, err := amqp.Dial(c.cfg.Rabbit.URL)
	if err != nil {
		return fmt.Errorf("amqp dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("amqp channel: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.pubChan = ch
	c.mu.Unlock()

	c.log.Info("rabbitmq connected", zap.String("url", c.cfg.Rabbit.URL))
	return nil
}

// watchReconnect blocks on NotifyClose of the underlying connection and
// re-dials with capped exponential backoff. It also re-declares any
// exchanges that the client owns and re-attaches every registered consumer.
func (c *Client) watchReconnect() {
	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()
		if conn == nil {
			return
		}

		notify := conn.NotifyClose(make(chan *amqp.Error, 1))
		select {
		case <-c.closed:
			return
		case reason, ok := <-notify:
			if !ok {
				return
			}
			c.log.Warn("rabbitmq connection closed", zap.Any("reason", reason))
		}

		backoff := time.Second
		for {
			select {
			case <-c.closed:
				return
			default:
			}
			if err := c.connect(); err != nil {
				c.log.Warn("rabbitmq reconnect failed", zap.Duration("backoff", backoff), zap.Error(err))
				time.Sleep(backoff)
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
				continue
			}
			// Replay topology.
			c.mu.RLock()
			for name, kind := range c.exchanges {
				if err := c.pubChan.ExchangeDeclare(name, kind, true, false, false, false, nil); err != nil {
					c.log.Error("rabbitmq re-declare exchange", zap.String("name", name), zap.Error(err))
				}
			}
			c.mu.RUnlock()

			c.consumersMu.Lock()
			specs := append([]consumerSpec(nil), c.consumers...)
			// Clear and rebuild — Consume re-appends.
			c.consumers = c.consumers[:0]
			c.consumersMu.Unlock()
			for _, s := range specs {
				if err := c.Consume(s.queue, s.exchange, s.routingKey, s.handler); err != nil {
					c.log.Error("rabbitmq re-consume",
						zap.String("queue", s.queue),
						zap.Error(err),
					)
				}
			}
			break
		}
	}
}

// DeclareExchange declares (or asserts) the given topic exchange. Calling it
// multiple times with the same args is safe.
func (c *Client) DeclareExchange(name, kind string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pubChan == nil {
		return errors.New("messaging not connected")
	}
	if err := c.pubChan.ExchangeDeclare(name, kind, true, false, false, false, nil); err != nil {
		return fmt.Errorf("exchange declare %s: %w", name, err)
	}
	c.exchanges[name] = kind
	return nil
}

// PublishJSON serializes payload as JSON and publishes it onto the exchange
// using the routing key. The publish is best-effort: when not connected, it
// returns an error but does not block waiting for reconnect.
func (c *Client) PublishJSON(ctx context.Context, exchange, routingKey string, payload any, headers amqp.Table) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	c.mu.RLock()
	ch := c.pubChan
	c.mu.RUnlock()
	if ch == nil {
		return errors.New("messaging unavailable")
	}

	to := c.cfg.Rabbit.PublishTimeout
	if to <= 0 {
		to = 5 * time.Second
	}
	pubCtx, cancel := context.WithTimeout(ctx, to)
	defer cancel()

	return ch.PublishWithContext(pubCtx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Headers:      headers,
		Body:         body,
	})
}

// Consume declares the given queue, binds it to (exchange, routingKey), and
// starts a goroutine that delivers messages to handler. Errors from the
// handler nack with requeue. The consumer is automatically re-registered on
// reconnect.
func (c *Client) Consume(queue, exchange, routingKey string, handler Handler) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return errors.New("messaging not connected")
	}

	// A dedicated channel per consumer so QoS/prefetch is independent.
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open consumer channel: %w", err)
	}
	if err := ch.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		return fmt.Errorf("declare exchange %s: %w", exchange, err)
	}
	q, err := ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("declare queue %s: %w", queue, err)
	}
	if err := ch.QueueBind(q.Name, routingKey, exchange, false, nil); err != nil {
		_ = ch.Close()
		return fmt.Errorf("bind queue %s -> %s/%s: %w", q.Name, exchange, routingKey, err)
	}
	if err := ch.Qos(8, 0, false); err != nil {
		_ = ch.Close()
		return fmt.Errorf("qos: %w", err)
	}

	deliveries, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("start consume: %w", err)
	}

	c.consumersMu.Lock()
	c.consumers = append(c.consumers, consumerSpec{
		queue:      queue,
		exchange:   exchange,
		routingKey: routingKey,
		handler:    handler,
	})
	c.consumersMu.Unlock()

	go func() {
		for d := range deliveries {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := handler(ctx, d.Body, d.Headers)
			cancel()
			if err != nil {
				c.log.Error("consumer handler error",
					zap.String("queue", queue),
					zap.String("routing_key", d.RoutingKey),
					zap.Error(err),
				)
				_ = d.Nack(false, true)
				continue
			}
			_ = d.Ack(false)
		}
		c.log.Warn("consumer deliveries channel closed", zap.String("queue", queue))
	}()

	return nil
}

// Close stops the reconnect loop and closes the connection. Idempotent.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closed)
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.pubChan != nil {
			_ = c.pubChan.Close()
			c.pubChan = nil
		}
		if c.conn != nil {
			err = c.conn.Close()
			c.conn = nil
		}
	})
	return err
}
