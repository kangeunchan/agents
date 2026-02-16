package runtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

func newGatewayClientForTest(t *testing.T, handler http.HandlerFunc) *GatewayClient {
	t.Helper()
	t.Setenv("OPENCLAW_STATE_DIR", t.TempDir())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := NewGatewayClient(srv.URL+"/rpc", "test-token")
	c.HTTPClient = srv.Client()
	return c
}

func readWSFrame(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	var frame map[string]any
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatalf("read websocket frame: %v", err)
	}
	return frame
}

func writeWSFrame(t *testing.T, conn *websocket.Conn, frame map[string]any) {
	t.Helper()
	if err := conn.WriteJSON(frame); err != nil {
		t.Fatalf("write websocket frame: %v", err)
	}
}

func completeWSConnectHandshake(t *testing.T, conn *websocket.Conn) map[string]any {
	return completeWSConnectHandshakeWithPayload(t, conn, nil)
}

func completeWSConnectHandshakeWithPayload(t *testing.T, conn *websocket.Conn, helloPayload map[string]any) map[string]any {
	t.Helper()

	writeWSFrame(t, conn, map[string]any{
		"type":    "event",
		"event":   "connect.challenge",
		"payload": map[string]any{"nonce": "nonce-1"},
	})

	connectReq := readWSFrame(t, conn)
	if connectReq["type"] != "req" || connectReq["method"] != "connect" {
		t.Fatalf("expected connect request, got: %#v", connectReq)
	}
	connectID, _ := connectReq["id"].(string)
	if strings.TrimSpace(connectID) == "" {
		t.Fatalf("connect request id is empty: %#v", connectReq)
	}
	params, ok := connectReq["params"].(map[string]any)
	if !ok {
		t.Fatalf("connect params missing: %#v", connectReq)
	}
	device, ok := params["device"].(map[string]any)
	if !ok {
		t.Fatalf("connect device missing: %#v", params)
	}
	for _, field := range []string{"id", "publicKey", "signature"} {
		value, _ := device[field].(string)
		if strings.TrimSpace(value) == "" {
			t.Fatalf("connect device.%s missing: %#v", field, device)
		}
	}

	if helloPayload == nil {
		helloPayload = map[string]any{"type": "hello-ok"}
	}
	writeWSFrame(t, conn, map[string]any{
		"type":    "res",
		"id":      connectID,
		"ok":      true,
		"payload": helloPayload,
	})
	return connectReq
}

func TestCallMarksUnsupportedTransport(t *testing.T) {
	client := newGatewayClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method Not Allowed"))
	})

	err := client.callHTTP(context.Background(), "config.get", map[string]any{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsRPCTransportUnsupported(err) {
		t.Fatalf("expected rpc transport unsupported error, got: %v", err)
	}
}

func TestGetConfigFallsBackToWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	client := newGatewayClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc" {
			http.NotFound(w, r)
			return
		}
		if websocket.IsWebSocketUpgrade(r) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("upgrade websocket: %v", err)
			}
			defer conn.Close()

			completeWSConnectHandshake(t, conn)

			req := readWSFrame(t, conn)
			if req["type"] != "req" || req["method"] != "config.get" {
				t.Fatalf("expected config.get request, got: %#v", req)
			}
			reqID, _ := req["id"].(string)

			writeWSFrame(t, conn, map[string]any{
				"type": "res",
				"id":   reqID,
				"ok":   true,
				"payload": map[string]any{
					"hash": "h1",
					"raw":  "{\n  \"gateway\": {\"mode\": \"local\"}\n}\n",
					"config": map[string]any{
						"gateway": map[string]any{"mode": "local"},
					},
				},
			})
			return
		}

		// Force HTTP JSON-RPC path to fail so ws fallback is exercised.
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method Not Allowed"))
	})

	cfg, err := client.GetConfig(context.Background())
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}
	gateway, ok := cfg["gateway"].(map[string]any)
	if !ok {
		t.Fatalf("gateway section missing: %#v", cfg)
	}
	if gateway["mode"] != "local" {
		t.Fatalf("unexpected gateway mode: %#v", gateway["mode"])
	}
}

func TestConnectStoresAndReusesDeviceToken(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var mu sync.Mutex
	var connectTokens []string
	handshakeCount := 0

	client := newGatewayClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc" {
			http.NotFound(w, r)
			return
		}
		if websocket.IsWebSocketUpgrade(r) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("upgrade websocket: %v", err)
			}
			defer conn.Close()

			mu.Lock()
			handshakeCount++
			currentHandshake := handshakeCount
			mu.Unlock()

			var hello map[string]any
			if currentHandshake == 1 {
				hello = map[string]any{
					"type": "hello-ok",
					"auth": map[string]any{
						"deviceToken": "device-token-1",
						"role":        "operator",
						"scopes":      []string{"operator.admin"},
					},
				}
			}
			connectReq := completeWSConnectHandshakeWithPayload(t, conn, hello)

			params, _ := connectReq["params"].(map[string]any)
			auth, _ := params["auth"].(map[string]any)
			authToken, _ := auth["token"].(string)
			mu.Lock()
			connectTokens = append(connectTokens, authToken)
			mu.Unlock()

			req := readWSFrame(t, conn)
			if req["method"] != "config.get" {
				t.Fatalf("expected config.get request, got: %#v", req)
			}
			reqID, _ := req["id"].(string)
			writeWSFrame(t, conn, map[string]any{
				"type": "res",
				"id":   reqID,
				"ok":   true,
				"payload": map[string]any{
					"hash": "h1",
					"raw":  "{\n  \"gateway\": {\"mode\": \"local\"}\n}\n",
					"config": map[string]any{
						"gateway": map[string]any{"mode": "local"},
					},
				},
			})
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method Not Allowed"))
	})

	if _, err := client.GetConfig(context.Background()); err != nil {
		t.Fatalf("first GetConfig returned error: %v", err)
	}
	if _, err := client.GetConfig(context.Background()); err != nil {
		t.Fatalf("second GetConfig returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(connectTokens) != 2 {
		t.Fatalf("expected 2 connect handshakes, got %d", len(connectTokens))
	}
	if connectTokens[0] != "test-token" {
		t.Fatalf("expected first connect to use shared token, got %q", connectTokens[0])
	}
	if connectTokens[1] != "device-token-1" {
		t.Fatalf("expected second connect to use stored device token, got %q", connectTokens[1])
	}
}

func TestApplyConfigUsesBaseHashFromConfigGet(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var mu sync.Mutex
	gotBaseHash := ""
	gotRaw := ""

	client := newGatewayClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc" {
			http.NotFound(w, r)
			return
		}
		if websocket.IsWebSocketUpgrade(r) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("upgrade websocket: %v", err)
			}
			defer conn.Close()

			completeWSConnectHandshake(t, conn)

			req := readWSFrame(t, conn)
			reqID, _ := req["id"].(string)
			method, _ := req["method"].(string)
			switch method {
			case "config.get":
				writeWSFrame(t, conn, map[string]any{
					"type": "res",
					"id":   reqID,
					"ok":   true,
					"payload": map[string]any{
						"hash": "live-hash-1",
						"raw":  "{\n  \"gateway\": {\"mode\": \"local\"}\n}\n",
						"config": map[string]any{
							"gateway": map[string]any{"mode": "local"},
						},
					},
				})
			case "config.apply":
				params, _ := req["params"].(map[string]any)
				baseHash, _ := params["baseHash"].(string)
				raw, _ := params["raw"].(string)
				mu.Lock()
				gotBaseHash = baseHash
				gotRaw = raw
				mu.Unlock()

				writeWSFrame(t, conn, map[string]any{
					"type":    "res",
					"id":      reqID,
					"ok":      true,
					"payload": map[string]any{"ok": true},
				})
			default:
				t.Fatalf("unexpected method: %s", method)
			}
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method Not Allowed"))
	})

	err := client.ApplyConfig(context.Background(), map[string]any{
		"gateway": map[string]any{
			"mode": "local",
			"bind": "loopback",
			"port": 18789,
		},
		"logging": map[string]any{"level": "info"},
	})
	if err != nil {
		t.Fatalf("ApplyConfig returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if gotBaseHash != "live-hash-1" {
		t.Fatalf("expected baseHash live-hash-1, got %q", gotBaseHash)
	}
	if !strings.Contains(gotRaw, "\"gateway\"") {
		t.Fatalf("expected raw json payload to contain gateway section, got: %q", gotRaw)
	}
}

func TestHealthUsesWebSocketFallback(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	client := newGatewayClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc" {
			http.NotFound(w, r)
			return
		}
		if websocket.IsWebSocketUpgrade(r) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("upgrade websocket: %v", err)
			}
			defer conn.Close()

			completeWSConnectHandshake(t, conn)

			req := readWSFrame(t, conn)
			if req["method"] != "health" {
				t.Fatalf("expected health request, got: %#v", req)
			}
			reqID, _ := req["id"].(string)
			writeWSFrame(t, conn, map[string]any{
				"type":    "res",
				"id":      reqID,
				"ok":      true,
				"payload": map[string]any{"status": "ok"},
			})
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method Not Allowed"))
	})

	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("expected health success, got: %v", err)
	}
}

func TestListPairingRequestsUsesWebSocketFallback(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	client := newGatewayClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc" {
			http.NotFound(w, r)
			return
		}
		if websocket.IsWebSocketUpgrade(r) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("upgrade websocket: %v", err)
			}
			defer conn.Close()

			completeWSConnectHandshake(t, conn)

			req := readWSFrame(t, conn)
			if req["method"] != "device.pair.list" {
				t.Fatalf("expected device.pair.list request, got: %#v", req)
			}
			reqID, _ := req["id"].(string)
			writeWSFrame(t, conn, map[string]any{
				"type": "res",
				"id":   reqID,
				"ok":   true,
				"payload": map[string]any{
					"pending": []map[string]any{
						{"requestId": "req-1", "deviceName": "web-ui"},
					},
					"paired": []map[string]any{
						{"deviceId": "dev-1", "deviceName": "operator-laptop"},
					},
				},
			})
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method Not Allowed"))
	})

	result, err := client.ListPairingRequests(context.Background())
	if err != nil {
		t.Fatalf("ListPairingRequests returned error: %v", err)
	}
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %#v", result)
	}
	pending, ok := resultMap["pending"].([]any)
	if !ok || len(pending) != 1 {
		t.Fatalf("expected 1 pending entry, got %#v", resultMap["pending"])
	}
}

func TestApprovePairingRequestFallbacksToIDParameter(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var mu sync.Mutex
	attempt := 0

	client := newGatewayClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc" {
			http.NotFound(w, r)
			return
		}
		if websocket.IsWebSocketUpgrade(r) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("upgrade websocket: %v", err)
			}
			defer conn.Close()

			completeWSConnectHandshake(t, conn)

			req := readWSFrame(t, conn)
			if req["method"] != "device.pair.approve" {
				t.Fatalf("expected device.pair.approve request, got: %#v", req)
			}
			reqID, _ := req["id"].(string)
			params, _ := req["params"].(map[string]any)

			mu.Lock()
			attempt++
			currentAttempt := attempt
			mu.Unlock()

			switch currentAttempt {
			case 1:
				if _, hasRequestID := params["requestId"]; !hasRequestID {
					t.Fatalf("first attempt should use requestId param, got: %#v", params)
				}
				writeWSFrame(t, conn, map[string]any{
					"type": "res",
					"id":   reqID,
					"ok":   false,
					"error": map[string]any{
						"code":    "invalid_params",
						"message": "missing field id",
					},
				})
			case 2:
				if _, hasID := params["id"]; !hasID {
					t.Fatalf("second attempt should use id param, got: %#v", params)
				}
				writeWSFrame(t, conn, map[string]any{
					"type":    "res",
					"id":      reqID,
					"ok":      true,
					"payload": map[string]any{"ok": true},
				})
			default:
				t.Fatalf("unexpected number of attempts: %d", currentAttempt)
			}
			return
		}

		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte("Method Not Allowed"))
	})

	if err := client.ApprovePairingRequest(context.Background(), "req-1"); err != nil {
		t.Fatalf("ApprovePairingRequest returned error: %v", err)
	}
}

func TestHealthRejectsHTMLFallback(t *testing.T) {
	client := newGatewayClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rpc":
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte("Method Not Allowed"))
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<!doctype html><html><body>ui</body></html>"))
		default:
			http.NotFound(w, r)
		}
	})

	err := client.Health(context.Background())
	if err == nil {
		t.Fatal("expected health error")
	}
	if !IsRPCTransportUnsupported(err) {
		t.Fatalf("expected unsupported rpc to be preserved, got: %v", err)
	}
	if !strings.Contains(err.Error(), "non-machine-readable") {
		t.Fatalf("expected non-machine-readable hint, got: %v", err)
	}
}

func TestHealthAcceptsPlainOKEndpoint(t *testing.T) {
	client := newGatewayClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rpc":
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte("Method Not Allowed"))
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		default:
			http.NotFound(w, r)
		}
	})

	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("expected health success, got: %v", err)
	}
}

func TestUIReachable(t *testing.T) {
	client := newGatewayClientForTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<!doctype html><html><body><openclaw-app></openclaw-app></body></html>"))
		default:
			http.NotFound(w, r)
		}
	})

	ok, err := client.UIReachable(context.Background())
	if err != nil {
		t.Fatalf("expected ui check success, got: %v", err)
	}
	if !ok {
		t.Fatal("expected ui reachable")
	}
}
