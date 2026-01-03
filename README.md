MQTT v5 client implementation in Go, WIP (not production ready) and currently supporting :
- Connection management with authentication and TLS
- Heartbeat/keep-alive mechanism
- Will messages and connect properties
- Qos 0/1/2message publishing
- Session management
- subscription management (subscribe only, unsub coming soon..)

Not yet supported : 
- receiving messages
- topic alias support
- auto reconnect
- message expiry
- enhanced auth ( AUTH packet )

Basic implementation : 

```go

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

err = client.Connect(ctx)
if err != nil {
    log.Printf("Failed to connect: %v", err)
    return
}

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
```
