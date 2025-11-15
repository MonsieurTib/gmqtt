package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/MonsieurTib/gmqtt"
)

func main() {
	ctx := context.Background()

	config := &gmqtt.ClientConfig{
		Broker:    "localhost:1883",
		ClientID:  "example-client",
		KeepAlive: 30 * time.Second,
		Logger: slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}),
		),
		ConnectProperties: &gmqtt.ConnectProperties{
			SessionExpiryInterval: 32,
			ReceiveMaximum:        100,
			UserProperties: []gmqtt.UserProperty{
				{Key: "userprop", Value: "hello prop value"},
			},
		},
		Will: &gmqtt.WillConfig{
			Topic:   "test",
			Message: "bye",
			QoS:     gmqtt.QoSAtMostOnce,
			Retain:  false,
			Properties: &gmqtt.WillProperties{
				WillDelayInterval:      30,
				MessageExpiryInterval:  3600,
				ContentType:            "text/plain",
				PayloadFormatIndicator: true,
				UserProperties: []gmqtt.UserProperty{
					{Key: "userprop", Value: "hello prop value"},
				},
			},
		},
	}

	client, err := gmqtt.NewClient(config)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return
	}
	/*
		defer func() {
			disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := client.Disconnect(disconnectCtx); err != nil {
				log.Printf("Error during cleanup disconnect: %v", err)
			}
		}()
	*/
	err = client.Connect(ctx)
	if err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}

	fmt.Println("Connected successfully!")
	client.Publish(ctx, gmqtt.Publish{
		Topic:   "hello/world",
		Qos:     gmqtt.QoSAtMostOnce,
		Payload: []byte("hello from example qos0 and retained"),
		Retain:  true,
	})

	response, err := client.Publish(ctx, gmqtt.Publish{
		Topic:   "hello/world",
		Qos:     gmqtt.QoSAtLeastOnce,
		Payload: []byte("hello from example qos1 and NOT retained"),
	})

	if err != nil {
		log.Printf("Failed to publish: %v", err)
	} else {
		log.Printf("Published successfully: %v", response)
	}

	time.Sleep(60 * time.Second)
	fmt.Println("Disconnecting")
	_ = client.Disconnect(ctx)
	fmt.Println("Disconnected")
	time.Sleep(5 * time.Second)
}
