package gmqtt

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type MQTTContainer struct {
	testcontainers.Container
	MQTTURI      string
	DashboardURL string
}

func TestCreateConnection(t *testing.T) {
	ctx := context.Background()
	container, err := setupEMQXContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup EMQX container: %v", err)
	}

	defer func() {
		if termErr := container.Terminate(ctx); termErr != nil {
			t.Logf("Failed to terminate container: %v", termErr)
		}
	}()

	t.Logf("EMQX Dashboard available at: %s", container.DashboardURL)
	t.Logf("Default login: admin/public")

	tests := []struct {
		name        string
		broker      string
		clientID    string
		tlsConfig   *tls.Config
		expectError bool
		expectedErr error
	}{
		{
			name:        "successful_connection_emqx",
			broker:      container.MQTTURI,
			clientID:    "test-connection",
			expectError: false,
		},
		{
			name:     "successful_connection_tls_hivemq",
			broker:   "broker.hivemq.com:8883",
			clientID: "test-tls-client",
			tlsConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ClientConfig{
				Broker:    tt.broker,
				ClientID:  tt.clientID,
				KeepAlive: 60 * time.Second,
				TLSConfig: tt.tlsConfig,
				ConnectProperties: &ConnectProperties{
					SessionExpiryInterval: 3600,
					ReceiveMaximum:        100,
					UserProperties: []UserProperty{
						{Key: "userprop", Value: "hello prop value"},
					},
				},
				Will: &WillConfig{
					Topic:   "test",
					Message: "bye",
					QoS:     QoSAtMostOnce,
					Retain:  false,
					Properties: &WillProperties{
						WillDelayInterval:      30,
						MessageExpiryInterval:  3600,
						ContentType:            "text/plain",
						PayloadFormatIndicator: true,
						UserProperties: []UserProperty{
							{Key: "userprop", Value: "hello prop value"},
						},
					},
				},
			}

			client, err := NewClient(config)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
			defer connectCancel()

			err = client.Connect(connectCtx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected connection to fail but it succeeded")
				} else {
					if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
						t.Errorf("Expected error %v, but got %v", tt.expectedErr, err)
					} else {
						t.Logf("Correctly failed with expected error type: %v", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Expected connection to succeed but it failed: %v", err)
				} else {
					t.Log("Successfully connected")
					if !client.IsConnected() {
						t.Errorf("Client.IsConnected() should be true after successful connection")
					}
					disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer disconnectCancel()
					if err := client.Disconnect(disconnectCtx); err != nil {
						t.Logf("Error during client disconnect: %v", err)
					}
				}
			}
		})
	}
}

func TestCreateConnectionWithAuth(t *testing.T) {
	ctx := context.Background()
	container, err := setupProtectedEMQXContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup protected EMQX container: %v", err)
	}

	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	t.Logf("EMQX Dashboard available at: %s", container.DashboardURL)
	t.Logf("Default login: admin/public")
	t.Logf("Test user created: testuser/testpass")

	tests := []struct {
		name        string
		username    string
		password    string
		clientID    string
		expectError bool
		expectedErr error
		description string
	}{
		{
			name:        "valid_credentials",
			username:    "testuser",
			password:    "testpass",
			clientID:    "test-valid-connection",
			expectError: false,
			expectedErr: nil,
			description: "Valid credentials should connect successfully",
		},
		{
			name:        "invalid_username",
			username:    "testuser2",
			password:    "testpass",
			clientID:    "test-invalid-username",
			expectError: true,
			expectedErr: ErrConnackNotAuthorized, // NOTE: EMQX returns "not authorized" for non-existent users, not "bad username", todo: open a issue
			description: "Invalid username should fail with ErrConnackNotAuthorized (EMQX behavior)",
		},
		{
			name:        "invalid_password",
			username:    "testuser",
			password:    "wrongpass",
			clientID:    "test-invalid-password",
			expectError: true,
			expectedErr: ErrConnackBadUsernameOrPassword,
			description: "Invalid password should fail with ErrConnackBadUsernameOrPassword",
		},
		{
			name:        "anonymous_connection",
			username:    "",
			password:    "",
			clientID:    "test-anonymous-conn",
			expectError: true,
			expectedErr: ErrConnackBadUsernameOrPassword,
			description: "Anonymous connection should fail with ErrConnackBadUsernameOrPassword",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ClientConfig{
				Broker:    container.MQTTURI,
				ClientID:  tt.clientID,
				Username:  tt.username,
				Password:  tt.password,
				KeepAlive: 60 * time.Second,
			}

			client, err := NewClient(config)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			err = client.Connect(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("%s: expected connection to fail but it succeeded", tt.description)
				} else {
					if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
						t.Errorf("%s: expected error %v, but got %v", tt.description, tt.expectedErr, err)
					} else {
						t.Logf("%s: correctly failed with expected error type: %v", tt.description, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("%s: expected connection to succeed but it failed: %v", tt.description, err)
				} else {
					t.Logf("%s: successfully connected", tt.description)
					if !client.connected {
						t.Errorf("Client.Connected should be true after successful connection")
					}
					defer client.Disconnect(ctx)
				}
			}
		})
	}
}

func TestReconnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	container, err := setupEMQXContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup EMQX container: %v", err)
	}
	defer func() {
		_ = container.Terminate(ctx)
	}()

	tests := []struct {
		name   string
		dropFn func(context.Context, *Client) error
	}{
		{
			name: "forced_close",
			dropFn: func(_ context.Context, c *Client) error {
				if c.conn != nil {
					_ = c.conn.Close()
				}
				return nil
			},
		},
		{
			name: "graceful_disconnect",
			dropFn: func(parent context.Context, c *Client) error {
				dctx, cancel := context.WithTimeout(parent, 5*time.Second)
				defer cancel()
				return c.Disconnect(dctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := createClient(t, container.MQTTURI, fmt.Sprintf("reconnect-e2e-%s", tt.name))

			if err := tt.dropFn(ctx, client); err != nil {
				t.Fatalf("dropFn returned error: %v", err)
			}

			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if !client.IsConnected() {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}

			if client.IsConnected() {
				t.Fatalf("client did not transition to disconnected in test %q", tt.name)
			}

			if err := client.Connect(ctx); err != nil {
				t.Fatalf("reconnect failed in test %q: %v", tt.name, err)
			}

			_ = client.Disconnect(ctx)
		})
	}
}

func TestPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	container, err := setupEMQXContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup EMQX container: %v", err)
	}
	defer func() {
		_ = container.Terminate(ctx)
	}()

	tests := []struct {
		name        string
		qos         QoS
		topic       string
		payload     []byte
		retain      bool
		expectError bool
		expectResp  bool
	}{
		{
			name:        "qos0_simple",
			qos:         QoSAtMostOnce,
			topic:       "test/qos0/simple",
			payload:     []byte("qos0 message"),
			retain:      false,
			expectError: false,
			expectResp:  false,
		},
		{
			name:        "qos1_simple",
			qos:         QoSAtLeastOnce,
			topic:       "test/qos1/simple",
			payload:     []byte("qos1 message"),
			retain:      false,
			expectError: false,
			expectResp:  true,
		},
		{
			name:        "qos1_with_retain",
			qos:         QoSAtLeastOnce,
			topic:       "test/qos1/retained",
			payload:     []byte("retained message"),
			retain:      true,
			expectError: false,
			expectResp:  true,
		},
		{
			name:        "qos0_empty_payload",
			qos:         QoSAtMostOnce,
			topic:       "test/qos0/empty",
			payload:     []byte{},
			retain:      false,
			expectError: false,
			expectResp:  false,
		},
		{
			name:        "qos1_large_payload",
			qos:         QoSAtLeastOnce,
			topic:       "test/qos1/large",
			payload:     make([]byte, 1024),
			retain:      false,
			expectError: false,
			expectResp:  true,
		},
		{
			name:        "qos2_simple",
			qos:         QoSExactlyOnce,
			topic:       "test/qos2/simple",
			payload:     []byte("qos2 message"),
			retain:      false,
			expectError: false,
			expectResp:  true,
		},
		{
			name:        "qos2_with_retain",
			qos:         QoSExactlyOnce,
			topic:       "test/qos2/retained",
			payload:     []byte("retained message"),
			retain:      true,
			expectError: false,
			expectResp:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ClientConfig{
				Broker:     container.MQTTURI,
				ClientID:   fmt.Sprintf("test-publish-%s", tt.name),
				KeepAlive:  60 * time.Second,
				CleanStart: true,
				Logger:     slog.Default(),
			}

			client, err := NewClient(config)
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			if err := client.Connect(connectCtx); err != nil {
				t.Fatalf("Failed to connect: %v", err)
			}
			defer client.Disconnect(ctx)

			publishCtx, publishCancel := context.WithTimeout(ctx, 5*time.Second)
			defer publishCancel()

			resp, err := client.Publish(publishCtx, Publish{
				Topic:   tt.topic,
				Qos:     tt.qos,
				Payload: tt.payload,
				Retain:  tt.retain,
			})

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectResp && resp == nil {
				t.Error("Expected response but got nil")
			}
			if !tt.expectResp && resp != nil {
				t.Errorf("Expected no response but got: %v", resp)
			}
		})
	}
}

func TestPublishConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	container, err := setupEMQXContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup EMQX container: %v", err)
	}
	defer func() {
		_ = container.Terminate(ctx)
	}()

	config := &ClientConfig{
		Broker:     container.MQTTURI,
		ClientID:   "test-concurrent-publisher",
		KeepAlive:  60 * time.Second,
		CleanStart: true,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Connect(connectCtx); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Disconnect(ctx)

	numMessages := 20
	done := make(chan error, numMessages)

	for i := range numMessages {
		go func(msgNum int) {
			publishCtx, publishCancel := context.WithTimeout(ctx, 10*time.Second)
			defer publishCancel()

			_, err := client.Publish(publishCtx, Publish{
				Topic:   fmt.Sprintf("test/concurrent/%d", msgNum),
				Qos:     QoSAtLeastOnce,
				Payload: fmt.Appendf(nil, "message %d", msgNum),
			})
			done <- err
		}(i)
	}

	failed := 0
	for i := range numMessages {
		if err := <-done; err != nil {
			t.Errorf("Publish %d failed: %v", i, err)
			failed++
		}
	}

	if failed > 0 {
		t.Errorf("%d out of %d concurrent publishes failed", failed, numMessages)
	}
}

func BenchmarkPublish(b *testing.B) {
	ctx := context.Background()
	container, err := setupEMQXContainer(ctx)
	if err != nil {
		b.Fatalf("Failed to setup EMQX container: %v", err)
	}
	defer func() {
		_ = container.Terminate(ctx)
	}()

	benchmarks := []struct {
		name        string
		qos         QoS
		payloadSize int
	}{
		{
			name:        "qos0_small_payload",
			qos:         QoSAtMostOnce,
			payloadSize: 100,
		},
		{
			name:        "qos0_medium_payload",
			qos:         QoSAtMostOnce,
			payloadSize: 1024,
		},
		{
			name:        "qos0_large_payload",
			qos:         QoSAtMostOnce,
			payloadSize: 10240,
		},
		{
			name:        "qos1_small_payload",
			qos:         QoSAtLeastOnce,
			payloadSize: 100,
		},
		{
			name:        "qos1_medium_payload",
			qos:         QoSAtLeastOnce,
			payloadSize: 1024,
		},
		{
			name:        "qos1_large_payload",
			qos:         QoSAtLeastOnce,
			payloadSize: 10240,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			config := &ClientConfig{
				Broker:     container.MQTTURI,
				ClientID:   fmt.Sprintf("bench-%s", bm.name),
				KeepAlive:  60 * time.Second,
				CleanStart: true,
			}

			client, err := NewClient(config)
			if err != nil {
				b.Fatalf("Failed to create client: %v", err)
			}

			connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			if err := client.Connect(connectCtx); err != nil {
				b.Fatalf("Failed to connect: %v", err)
			}
			defer client.Disconnect(ctx)

			payload := make([]byte, bm.payloadSize)
			for i := range payload {
				payload[i] = byte(i % 256)
			}

			b.ResetTimer()
			for i := 0; b.Loop(); i++ {
				publishCtx, publishCancel := context.WithTimeout(ctx, 5*time.Second)
				_, err := client.Publish(publishCtx, Publish{
					Topic:   fmt.Sprintf("bench/test/%d", i),
					Qos:     bm.qos,
					Payload: payload,
				})
				publishCancel()

				if err != nil {
					b.Fatalf("Publish failed at iteration %d: %v", i, err)
				}
			}
			b.StopTimer()
		})
	}
}

func BenchmarkPublishParallel(b *testing.B) {
	ctx := context.Background()
	container, err := setupEMQXContainer(ctx)
	if err != nil {
		b.Fatalf("Failed to setup EMQX container: %v", err)
	}
	defer func() {
		_ = container.Terminate(ctx)
	}()

	benchmarks := []struct {
		name        string
		qos         QoS
		payloadSize int
	}{
		{
			name:        "qos0_small_payload",
			qos:         QoSAtMostOnce,
			payloadSize: 100,
		},
		{
			name:        "qos1_small_payload",
			qos:         QoSAtLeastOnce,
			payloadSize: 100,
		},
		{
			name:        "qos1_medium_payload",
			qos:         QoSAtLeastOnce,
			payloadSize: 1024,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			config := &ClientConfig{
				Broker:     container.MQTTURI,
				ClientID:   fmt.Sprintf("bench-parallel-%s", bm.name),
				KeepAlive:  60 * time.Second,
				CleanStart: true,
			}

			client, err := NewClient(config)
			if err != nil {
				b.Fatalf("Failed to create client: %v", err)
			}

			connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			if err := client.Connect(connectCtx); err != nil {
				b.Fatalf("Failed to connect: %v", err)
			}
			defer client.Disconnect(ctx)

			payload := make([]byte, bm.payloadSize)
			for i := range payload {
				payload[i] = byte(i % 256)
			}

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					publishCtx, publishCancel := context.WithTimeout(ctx, 5*time.Second)
					_, err := client.Publish(publishCtx, Publish{
						Topic:   fmt.Sprintf("bench/parallel/%d", i),
						Qos:     bm.qos,
						Payload: payload,
					})
					publishCancel()

					if err != nil {
						b.Errorf("Publish failed: %v", err)
					}
					i++
				}
			})
			b.StopTimer()
		})
	}
}

func setupProtectedEMQXContainer(ctx context.Context) (*MQTTContainer, error) {
	_, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name: "emqx-test-network",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	redisContainer, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        "redis:7-alpine",
				ExposedPorts: []string{"6379/tcp"},
				WaitingFor:   wait.ForListeningPort("6379/tcp"),
				Networks:     []string{"emqx-test-network"},
				NetworkAliases: map[string][]string{
					"emqx-test-network": {"redis"},
				},
			},
			Started: true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to setup Redis: %w", err)
	}

	time.Sleep(2 * time.Second)

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "emqx/emqx:5.3.0",
			ExposedPorts: []string{"1883/tcp", "18083/tcp"},
			Networks:     []string{"emqx-test-network"},
			WaitingFor: wait.ForAll(
				wait.ForLog("EMQX").WithStartupTimeout(10*time.Second),
				wait.ForListeningPort("1883/tcp"),
				wait.ForListeningPort("18083/tcp"),
			),
			Env: map[string]string{
				"EMQX_ALLOW_ANONYMOUS":                                           "false",
				"EMQX_LOG__LEVEL":                                                "warning",
				"EMQX_AUTHENTICATION__1__MECHANISM":                              "password_based",
				"EMQX_AUTHENTICATION__1__BACKEND":                                "redis",
				"EMQX_AUTHENTICATION__1__REDIS_TYPE":                             "single",
				"EMQX_AUTHENTICATION__1__SERVER":                                 "redis:6379",
				"EMQX_AUTHENTICATION__1__PASSWORD_HASH_ALGORITHM__NAME":          "sha256",
				"EMQX_AUTHENTICATION__1__PASSWORD_HASH_ALGORITHM__SALT_POSITION": "suffix",
				"EMQX_AUTHENTICATION__1__CMD":                                    "HMGET mqtt_user:${username} password_hash salt is_superuser",
				"EMQX_AUTHENTICATION__1__DATABASE":                               "0",
				"EMQX_AUTHENTICATION__1__AUTO_RECONNECT":                         "true",
			},
		},
		Started: true,
	})
	if err != nil {
		return nil, err
	}

	mqttPort, err := container.MappedPort(ctx, "1883")
	if err != nil {
		return nil, err
	}

	dashboardPort, err := container.MappedPort(ctx, "18083")
	if err != nil {
		return nil, err
	}

	hostIP, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mqttURI := fmt.Sprintf("%s:%s", hostIP, mqttPort.Port())
	dashboardURL := fmt.Sprintf("http://%s:%s", hostIP, dashboardPort.Port())

	err = createRedisUser(ctx, redisContainer)
	if err != nil {
		return nil, fmt.Errorf("failed to populate Redis user data: %w", err)
	}

	return &MQTTContainer{
		Container:    container,
		MQTTURI:      mqttURI,
		DashboardURL: dashboardURL,
	}, nil
}

func setupEMQXContainer(ctx context.Context) (*MQTTContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "emqx/emqx:5.3.0",
		ExposedPorts: []string{"1883/tcp", "18083/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("EMQX").WithStartupTimeout(10*time.Second),
			wait.ForListeningPort("1883/tcp"),
			wait.ForListeningPort("18083/tcp"),
		),
		Env: map[string]string{
			"EMQX_ALLOW_ANONYMOUS": "true",
			"EMQX_LOG__LEVEL":      "warning",
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, err
	}

	mqttPort, err := container.MappedPort(ctx, "1883")
	if err != nil {
		return nil, err
	}

	dashboardPort, err := container.MappedPort(ctx, "18083")
	if err != nil {
		return nil, err
	}

	hostIP, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	mqttURI := fmt.Sprintf("%s:%s", hostIP, mqttPort.Port())
	dashboardURL := fmt.Sprintf("http://%s:%s", hostIP, dashboardPort.Port())

	return &MQTTContainer{
		Container:    container,
		MQTTURI:      mqttURI,
		DashboardURL: dashboardURL,
	}, nil
}

func createClient(t *testing.T, brokerURI string, clientID string) *Client {
	ctx := context.Background()
	config := &ClientConfig{
		Broker:    brokerURI,
		ClientID:  clientID,
		KeepAlive: 60 * time.Second,
		ConnectProperties: &ConnectProperties{
			SessionExpiryInterval: 3600,
			ReceiveMaximum:        100,
			UserProperties: []UserProperty{
				{Key: "userprop", Value: "hello prop value"},
			},
		},
		Will: &WillConfig{
			Topic:   "test",
			Message: "bye",
			QoS:     QoSAtMostOnce,
			Retain:  false,
			Properties: &WillProperties{
				WillDelayInterval:      30,
				MessageExpiryInterval:  3600,
				ContentType:            "text/plain",
				PayloadFormatIndicator: true,
				UserProperties: []UserProperty{
					{Key: "userprop", Value: "hello prop value"},
				},
			},
		},
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("can not create client : %v", err)
	}

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("can not connect client: %v", err)
	}

	return client
}

func createRedisUser(ctx context.Context, redisContainer testcontainers.Container) error {
	h := sha256.New()
	h.Write([]byte("testpass" + "salt"))
	passwordHash := fmt.Sprintf("%x", h.Sum(nil))

	commands := [][]string{
		{"redis-cli", "HSET", "mqtt_user:testuser", "password_hash", passwordHash},
		{"redis-cli", "HSET", "mqtt_user:testuser", "salt", "salt"},
		{"redis-cli", "HSET", "mqtt_user:testuser", "is_superuser", "false"},
		{"redis-cli", "HGETALL", "mqtt_user:testuser"},
	}

	for i, cmd := range commands {
		exitCode, reader, err := redisContainer.Exec(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to execute Redis command %v: %w", cmd, err)
		}
		if exitCode != 0 {
			return fmt.Errorf("Redis command %v failed with exit code %d", cmd, exitCode)
		}

		// For the HGETALL command, read and log the output
		if i == len(commands)-1 {
			output, err := io.ReadAll(reader)
			if err == nil {
				fmt.Printf("Redis user data: %s\n", string(output))
			}
		}
	}

	return nil
}
