package gmqtt

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
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
