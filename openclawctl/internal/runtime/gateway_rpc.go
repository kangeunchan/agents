package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type GatewayClient struct {
	RPCURL     string
	Token      string
	Password   string
	HTTPClient *http.Client
}

type ConfigSnapshot struct {
	Config map[string]any
	Raw    string
	Hash   string
}

var ErrRPCTransportUnsupported = errors.New("gateway rpc transport unsupported")

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error,omitempty"`
}

type wsErrorShape struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type wsFrame struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Event   string          `json:"event,omitempty"`
	Ok      bool            `json:"ok,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *wsErrorShape   `json:"error,omitempty"`
}

type wsRequestFrame struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type RPCStatusError struct {
	Method     string
	StatusCode int
	Body       string
}

func (e *RPCStatusError) Error() string {
	return fmt.Sprintf("rpc %s returned status %d: %s", e.Method, e.StatusCode, e.Body)
}

func IsRPCTransportUnsupported(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrRPCTransportUnsupported) {
		return true
	}

	var statusErr *RPCStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.StatusCode {
		case http.StatusMethodNotAllowed, http.StatusNotFound, http.StatusUpgradeRequired:
			return true
		}
	}
	return false
}

func NewGatewayClient(rpcURL, token string) *GatewayClient {
	if strings.TrimSpace(rpcURL) == "" {
		rpcURL = os.Getenv("OPENCLAW_GATEWAY_RPC_URL")
	}
	if strings.TrimSpace(rpcURL) == "" {
		rpcURL = "http://127.0.0.1:18789/rpc"
	}
	if strings.TrimSpace(token) == "" {
		token = os.Getenv("OPENCLAW_GATEWAY_TOKEN")
	}
	password := os.Getenv("OPENCLAW_GATEWAY_PASSWORD")

	return &GatewayClient{
		RPCURL:   rpcURL,
		Token:    token,
		Password: password,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *GatewayClient) GetConfigSnapshot(ctx context.Context) (*ConfigSnapshot, error) {
	var raw json.RawMessage
	if err := c.call(ctx, "config.get", map[string]any{}, &raw); err != nil {
		return nil, fmt.Errorf("rpc config.get failed: %w", err)
	}

	var direct map[string]any
	if err := json.Unmarshal(raw, &direct); err != nil {
		return nil, fmt.Errorf("decode config.get response: %w", err)
	}

	snapshot := &ConfigSnapshot{}
	if hash, ok := direct["hash"].(string); ok {
		snapshot.Hash = strings.TrimSpace(hash)
	}
	if rawConfig, ok := direct["raw"].(string); ok {
		snapshot.Raw = rawConfig
	}
	if cfg, ok := direct["config"].(map[string]any); ok {
		snapshot.Config = cfg
	}
	if snapshot.Config == nil {
		if _, hasGateway := direct["gateway"]; hasGateway {
			snapshot.Config = direct
		}
	}
	if snapshot.Config == nil {
		return nil, fmt.Errorf("unexpected config.get response: %s", string(raw))
	}
	if strings.TrimSpace(snapshot.Raw) == "" {
		rawBytes, err := toPrettyJSON(snapshot.Config)
		if err != nil {
			return nil, fmt.Errorf("marshal config.get config to raw json: %w", err)
		}
		snapshot.Raw = string(rawBytes)
	}
	return snapshot, nil
}

func (c *GatewayClient) GetConfig(ctx context.Context) (map[string]any, error) {
	snapshot, err := c.GetConfigSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snapshot.Config, nil
}

func (c *GatewayClient) ApplyConfig(ctx context.Context, cfg map[string]any) error {
	snapshot, err := c.GetConfigSnapshot(ctx)
	if err != nil {
		return fmt.Errorf("read live config snapshot before apply: %w", err)
	}

	rawJSON, err := toPrettyJSON(cfg)
	if err != nil {
		return fmt.Errorf("render desired config json: %w", err)
	}
	params := map[string]any{
		"raw": string(rawJSON),
	}
	if snapshot.Hash != "" {
		params["baseHash"] = snapshot.Hash
	}

	if err := c.call(ctx, "config.apply", params, nil); err != nil {
		return fmt.Errorf("rpc config.apply failed: %w", err)
	}
	return nil
}

func (c *GatewayClient) Reload(ctx context.Context) error {
	if err := c.call(ctx, "gateway.reload", map[string]any{}, nil); err != nil {
		if isUnsupportedMethodError(err) || IsRPCTransportUnsupported(err) {
			return nil
		}
		return fmt.Errorf("rpc gateway.reload failed: %w", err)
	}
	return nil
}

func (c *GatewayClient) ListPairingRequests(ctx context.Context) (any, error) {
	var result any
	if err := c.call(ctx, "device.pair.list", map[string]any{}, &result); err != nil {
		return nil, fmt.Errorf("rpc device.pair.list failed: %w", err)
	}
	return result, nil
}

func (c *GatewayClient) ApprovePairingRequest(ctx context.Context, requestID string) error {
	return c.pairingDecision(ctx, "device.pair.approve", requestID)
}

func (c *GatewayClient) RejectPairingRequest(ctx context.Context, requestID string) error {
	return c.pairingDecision(ctx, "device.pair.reject", requestID)
}

func (c *GatewayClient) pairingDecision(ctx context.Context, method string, requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return fmt.Errorf("request id is required")
	}

	attempts := []any{
		map[string]any{"requestId": requestID},
		map[string]any{"id": requestID},
	}

	var lastErr error
	for _, params := range attempts {
		if err := c.call(ctx, method, params, nil); err != nil {
			lastErr = err
			if isPairingParamMismatch(err) {
				continue
			}
			return fmt.Errorf("rpc %s failed: %w", method, err)
		}
		return nil
	}

	return fmt.Errorf("rpc %s failed: %w", method, lastErr)
}

func (c *GatewayClient) Health(ctx context.Context) error {
	rpcErr := c.call(ctx, "health", map[string]any{}, nil)
	if rpcErr == nil {
		return nil
	}

	httpErr := c.healthFromEndpoint(ctx)
	if httpErr == nil {
		return nil
	}

	return fmt.Errorf("health check failed: %w (endpoint: %v)", rpcErr, httpErr)
}

func (c *GatewayClient) call(ctx context.Context, method string, params any, out any) error {
	httpErr := c.callHTTP(ctx, method, params, out)
	if httpErr == nil {
		return nil
	}
	if !IsRPCTransportUnsupported(httpErr) {
		return httpErr
	}

	wsErr := c.callWS(ctx, method, params, out)
	if wsErr == nil {
		return nil
	}
	if IsRPCTransportUnsupported(wsErr) {
		return fmt.Errorf("%w: http=%v; ws=%v", ErrRPCTransportUnsupported, httpErr, wsErr)
	}
	return fmt.Errorf("http rpc unsupported (%v); websocket fallback failed: %w", httpErr, wsErr)
}

func (c *GatewayClient) callHTTP(ctx context.Context, method string, params any, out any) error {
	payload := rpcRequest{
		JSONRPC: "2.0",
		ID:      time.Now().UnixNano(),
		Method:  method,
		Params:  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RPCURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.attachAuth(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("rpc %s request failed: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("read rpc response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		statusErr := &RPCStatusError{
			Method:     method,
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(respBody)),
		}
		if IsRPCTransportUnsupported(statusErr) {
			return fmt.Errorf("%w: %w", ErrRPCTransportUnsupported, statusErr)
		}
		return statusErr
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		if isHTMLResponse(resp.Header.Get("Content-Type"), respBody) {
			return fmt.Errorf("%w: rpc %s returned HTML instead of JSON", ErrRPCTransportUnsupported, method)
		}
		return fmt.Errorf("decode rpc response: %w", err)
	}
	if rpcResp.Error != nil {
		return fmt.Errorf("rpc %s error %d: %s", method, rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if out != nil && len(rpcResp.Result) > 0 {
		if err := json.Unmarshal(rpcResp.Result, out); err != nil {
			return fmt.Errorf("decode rpc result: %w", err)
		}
	}
	return nil
}

func (c *GatewayClient) callWS(ctx context.Context, method string, params any, out any) error {
	wsURL, err := c.wsURL()
	if err != nil {
		return fmt.Errorf("resolve websocket url: %w", err)
	}

	header := http.Header{}
	c.attachAuthHeaders(header)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, resp, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			_ = resp.Body.Close()
			statusErr := &RPCStatusError{
				Method:     method,
				StatusCode: resp.StatusCode,
				Body:       strings.TrimSpace(string(respBody)),
			}
			if IsRPCTransportUnsupported(statusErr) {
				return fmt.Errorf("%w: %w", ErrRPCTransportUnsupported, statusErr)
			}
			return statusErr
		}
		return fmt.Errorf("ws dial failed: %w", err)
	}
	defer conn.Close()

	nonce, err := c.readConnectChallenge(ctx, conn, 750*time.Millisecond)
	if err != nil {
		return err
	}

	connectID := randomFrameID()
	if err := c.wsWriteJSON(ctx, conn, wsRequestFrame{
		Type:   "req",
		ID:     connectID,
		Method: "connect",
		Params: c.wsConnectParams(nonce),
	}); err != nil {
		return fmt.Errorf("send connect request: %w", err)
	}
	if _, err := c.waitForWSResponse(ctx, conn, connectID, "connect"); err != nil {
		return fmt.Errorf("connect request failed: %w", err)
	}

	reqID := randomFrameID()
	if err := c.wsWriteJSON(ctx, conn, wsRequestFrame{
		Type:   "req",
		ID:     reqID,
		Method: method,
		Params: params,
	}); err != nil {
		return fmt.Errorf("send rpc request: %w", err)
	}
	payload, err := c.waitForWSResponse(ctx, conn, reqID, method)
	if err != nil {
		return err
	}

	if out != nil && len(payload) > 0 && string(payload) != "null" {
		if err := json.Unmarshal(payload, out); err != nil {
			return fmt.Errorf("decode rpc result: %w", err)
		}
	}
	return nil
}

func (c *GatewayClient) readConnectChallenge(ctx context.Context, conn *websocket.Conn, timeout time.Duration) (string, error) {
	frame, err := c.wsReadFrame(ctx, conn, timeout)
	if err != nil {
		if isTimeoutError(err) {
			return "", nil
		}
		return "", fmt.Errorf("read connect challenge: %w", err)
	}
	if frame.Type != "event" || frame.Event != "connect.challenge" {
		return "", nil
	}

	var payload struct {
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(frame.Payload, &payload); err != nil {
		return "", nil
	}
	return strings.TrimSpace(payload.Nonce), nil
}

func (c *GatewayClient) waitForWSResponse(ctx context.Context, conn *websocket.Conn, id string, method string) (json.RawMessage, error) {
	for {
		frame, err := c.wsReadFrame(ctx, conn, 0)
		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}
		if frame.Type != "res" || frame.ID != id {
			continue
		}
		if frame.Ok {
			return frame.Payload, nil
		}

		code := ""
		message := "request failed"
		if frame.Error != nil {
			code = strings.TrimSpace(frame.Error.Code)
			if strings.TrimSpace(frame.Error.Message) != "" {
				message = strings.TrimSpace(frame.Error.Message)
			}
		}
		if code != "" {
			return nil, fmt.Errorf("rpc %s error %s: %s", method, code, message)
		}
		return nil, fmt.Errorf("rpc %s error: %s", method, message)
	}
}

func (c *GatewayClient) wsReadFrame(ctx context.Context, conn *websocket.Conn, timeout time.Duration) (wsFrame, error) {
	deadline := time.Now().Add(15 * time.Second)
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return wsFrame{}, fmt.Errorf("set read deadline: %w", err)
	}

	_, payload, err := conn.ReadMessage()
	if err != nil {
		return wsFrame{}, err
	}

	var frame wsFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return wsFrame{}, fmt.Errorf("decode frame: %w", err)
	}
	return frame, nil
}

func (c *GatewayClient) wsWriteJSON(ctx context.Context, conn *websocket.Conn, frame wsRequestFrame) error {
	deadline := time.Now().Add(15 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}
	if err := conn.WriteJSON(frame); err != nil {
		return err
	}
	return nil
}

func (c *GatewayClient) wsConnectParams(_ string) map[string]any {
	client := map[string]any{
		"id":         "cli",
		"version":    "openclawctl-dev",
		"platform":   goruntime.GOOS,
		"mode":       "cli",
		"instanceId": randomFrameID(),
	}
	params := map[string]any{
		"minProtocol": 3,
		"maxProtocol": 3,
		"client":      client,
		"role":        "operator",
		"scopes":      []string{"operator.admin", "operator.approvals", "operator.pairing"},
	}

	auth := map[string]any{}
	if strings.TrimSpace(c.Token) != "" {
		auth["token"] = strings.TrimSpace(c.Token)
	}
	if strings.TrimSpace(c.Password) != "" {
		auth["password"] = strings.TrimSpace(c.Password)
	}
	if len(auth) > 0 {
		params["auth"] = auth
	}
	return params
}

func (c *GatewayClient) UIReachable(ctx context.Context) (bool, error) {
	uiURL, err := c.uiURL()
	if err != nil {
		return false, err
	}

	tryRequest := func(withAuth bool) (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, uiURL, nil)
		if err != nil {
			return false, fmt.Errorf("create request: %w", err)
		}
		if withAuth {
			c.attachAuth(req)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return false, fmt.Errorf("request ui: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return false, fmt.Errorf("ui returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		if isHTMLResponse(resp.Header.Get("Content-Type"), body) {
			return true, nil
		}
		return false, fmt.Errorf("ui returned non-html content-type: %s", resp.Header.Get("Content-Type"))
	}

	ok, noAuthErr := tryRequest(false)
	if noAuthErr == nil {
		return ok, nil
	}
	if strings.TrimSpace(c.Token) == "" {
		return false, noAuthErr
	}
	ok, withAuthErr := tryRequest(true)
	if withAuthErr == nil {
		return ok, nil
	}
	return false, fmt.Errorf("ui probe failed (no-auth: %v, with-auth: %v)", noAuthErr, withAuthErr)
}

func (c *GatewayClient) healthFromEndpoint(ctx context.Context) error {
	healthURL, err := c.healthURL()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	raw := strings.TrimSpace(string(body))
	rawLower := strings.ToLower(raw)
	if strings.Contains(contentType, "application/json") {
		if strings.Contains(rawLower, "\"status\":\"ok\"") || strings.Contains(rawLower, "\"status\":\"healthy\"") {
			return nil
		}
		if strings.Contains(rawLower, "\"ok\":true") || strings.Contains(rawLower, "\"healthy\":true") {
			return nil
		}
		return fmt.Errorf("json body does not indicate healthy: %s", raw)
	}

	if rawLower == "ok" || rawLower == "healthy" {
		return nil
	}

	return fmt.Errorf("non-machine-readable health response (content-type: %s)", resp.Header.Get("Content-Type"))
}

func (c *GatewayClient) attachAuth(req *http.Request) {
	if req == nil {
		return
	}
	c.attachAuthHeaders(req.Header)
}

func (c *GatewayClient) attachAuthHeaders(headers http.Header) {
	if headers == nil {
		return
	}
	if strings.TrimSpace(c.Token) != "" {
		token := strings.TrimSpace(c.Token)
		headers.Set("Authorization", "Bearer "+token)
		headers.Set("X-OpenClaw-Token", token)
	}
}

func (c *GatewayClient) healthURL() (string, error) {
	return c.endpointURL("health")
}

func (c *GatewayClient) uiURL() (string, error) {
	return c.endpointURL("")
}

func (c *GatewayClient) wsURL() (string, error) {
	u, err := url.Parse(c.RPCURL)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(u.Scheme)) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// keep as-is
	default:
		return "", fmt.Errorf("unsupported rpc url scheme: %s", u.Scheme)
	}
	if strings.TrimSpace(u.Path) == "" {
		u.Path = "/rpc"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func (c *GatewayClient) endpointURL(leaf string) (string, error) {
	u, err := url.Parse(c.RPCURL)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(leaf) == "" {
		u.Path = "/"
	} else {
		u.Path = path.Join(path.Dir(u.Path), leaf)
		if !strings.HasPrefix(u.Path, "/") {
			u.Path = "/" + u.Path
		}
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func isHTMLResponse(contentType string, body []byte) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "text/html") {
		return true
	}
	text := strings.ToLower(string(body))
	return strings.Contains(text, "<!doctype html") || strings.Contains(text, "<html")
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "i/o timeout")
}

func isUnsupportedMethodError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "unknown method") || strings.Contains(text, "method not found")
}

func isPairingParamMismatch(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "invalid params") ||
		strings.Contains(text, "missing parameter") ||
		strings.Contains(text, "missing field") ||
		strings.Contains(text, "expected field")
}

func randomFrameID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

func toPrettyJSON(v any) ([]byte, error) {
	bytesData, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(bytesData, '\n'), nil
}
