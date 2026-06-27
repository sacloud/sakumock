package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sacloud/sakumock/eventbus"
	"github.com/sacloud/sakumock/simplenotification"
)

// InspectCmd groups subcommands that query or drive a running sakumock's
// mock-only inspection endpoints (/_sakumock/).
type InspectCmd struct {
	Eventbus           InspectEventbusCmd           `cmd:"" name:"eventbus" help:"Inspect EventBus mock state"`
	Simplenotification InspectSimplenotificationCmd `cmd:"" name:"simplenotification" help:"Inspect Simple Notification mock state"`
}

// --- EventBus ---

type InspectEventbusCmd struct {
	Addr            string                    `help:"EventBus server address" default:"http://127.0.0.1:18085/" env:"SAKURA_ENDPOINTS_EVENTBUS"`
	Deliveries      InspectDeliveriesCmd      `cmd:"" name:"deliveries" help:"List recorded firings"`
	ClearDeliveries InspectClearDeliveriesCmd `cmd:"" name:"clear-deliveries" help:"Clear recorded firings"`
	InjectEvent     InspectInjectEventCmd     `cmd:"" name:"inject-event" help:"Inject an event and fire matching triggers"`
	Tick            InspectTickCmd            `cmd:"" name:"tick" help:"Evaluate schedules and fire those due"`
}

type InspectDeliveriesCmd struct {
	Addr string `kong:"-"`
}

func (c *InspectDeliveriesCmd) Run(ctx context.Context) error {
	ds, err := eventbus.NewInspectionClient(c.Addr).Deliveries(ctx)
	if err != nil {
		return err
	}
	return writeJSON(ds)
}

type InspectClearDeliveriesCmd struct {
	Addr string `kong:"-"`
}

func (c *InspectClearDeliveriesCmd) Run(ctx context.Context) error {
	return eventbus.NewInspectionClient(c.Addr).ClearDeliveries(ctx)
}

type InspectInjectEventCmd struct {
	Addr   string            `kong:"-"`
	Source string            `required:"" help:"Event source identifier"`
	Type   string            `help:"Event type"`
	Attr   map[string]string `help:"Event attributes (KEY=VALUE, repeatable)" mapsep:","`
	Data   string            `help:"Event data as a JSON string"`
}

func (c *InspectInjectEventCmd) Run(ctx context.Context) error {
	ev := eventbus.Event{
		Source: c.Source,
		Type:   c.Type,
	}
	if len(c.Attr) > 0 {
		ev.Attributes = make(map[string]any, len(c.Attr))
		for k, v := range c.Attr {
			ev.Attributes[k] = v
		}
	}
	if c.Data != "" {
		ev.Data = json.RawMessage(c.Data)
	}
	ds, err := eventbus.NewInspectionClient(c.Addr).InjectEvent(ctx, ev)
	if err != nil {
		return err
	}
	return writeJSON(ds)
}

type InspectTickCmd struct {
	Addr string    `kong:"-"`
	At   time.Time `help:"Evaluation time (RFC3339); defaults to server's current time" format:"2006-01-02T15:04:05Z07:00"`
}

func (c *InspectTickCmd) Run(ctx context.Context) error {
	ds, err := eventbus.NewInspectionClient(c.Addr).Tick(ctx, c.At)
	if err != nil {
		return err
	}
	return writeJSON(ds)
}

// BeforeApply propagates the parent Addr to each leaf command so the leaf
// does not need its own --addr flag.
func (c *InspectEventbusCmd) BeforeApply() error {
	c.Deliveries.Addr = c.Addr
	c.ClearDeliveries.Addr = c.Addr
	c.InjectEvent.Addr = c.Addr
	c.Tick.Addr = c.Addr
	return nil
}

// --- Simple Notification ---

type InspectSimplenotificationCmd struct {
	Addr          string                  `help:"Simple Notification server address" default:"http://127.0.0.1:18083" env:"SAKURA_ENDPOINTS_SIMPLE_NOTIFICATION"`
	Messages      InspectMessagesCmd      `cmd:"" name:"messages" help:"List accepted messages"`
	ClearMessages InspectClearMessagesCmd `cmd:"" name:"clear-messages" help:"Clear accepted messages"`
}

type InspectMessagesCmd struct {
	Addr string `kong:"-"`
}

func (c *InspectMessagesCmd) Run(ctx context.Context) error {
	msgs, err := simplenotification.NewInspectionClient(c.Addr).Messages(ctx)
	if err != nil {
		return err
	}
	return writeJSON(msgs)
}

type InspectClearMessagesCmd struct {
	Addr string `kong:"-"`
}

func (c *InspectClearMessagesCmd) Run(ctx context.Context) error {
	return simplenotification.NewInspectionClient(c.Addr).ClearMessages(ctx)
}

func (c *InspectSimplenotificationCmd) BeforeApply() error {
	c.Messages.Addr = c.Addr
	c.ClearMessages.Addr = c.Addr
	return nil
}

// --- Helpers ---

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	return nil
}
