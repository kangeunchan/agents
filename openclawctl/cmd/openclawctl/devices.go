package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func runDevices(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("devices subcommand required (list|approve|reject)")
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "list":
		return runDevicesList(args[1:])
	case "approve":
		return runDevicesDecision("approve", args[1:])
	case "reject":
		return runDevicesDecision("reject", args[1:])
	default:
		return fmt.Errorf("unknown devices subcommand %q (expected list|approve|reject)", args[0])
	}
}

func runDevicesList(args []string) error {
	jsonOut, rest, err := parseJSONFlag(args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return fmt.Errorf("usage: openclawctl devices list [--json]")
	}

	manifest, err := loadManifest()
	if err != nil {
		return err
	}
	containerName := currentContainerName(manifest)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := gatewayClient().ListPairingRequests(ctx)
	if err != nil {
		fallbackResult, fallbackErr := listPairingRequestsViaContainer(ctx, containerName)
		if fallbackErr != nil {
			return fmt.Errorf("pairing list failed: rpc=%v; container-fallback=%w", err, fallbackErr)
		}
		outWarnf("RPC pairing list unavailable, used container fallback: %v", err)
		result = fallbackResult
	}

	if jsonOut {
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("encode json output: %w", err)
		}
		fmt.Println(string(encoded))
		return nil
	}

	pending, paired := splitPairingEntries(result)
	fmt.Printf("%s %s\n", keyLabel("pending:"), accent(strconv.Itoa(len(pending))))
	printPairingEntries(pending)
	fmt.Printf("%s %s\n", keyLabel("paired:"), accent(strconv.Itoa(len(paired))))
	printPairingEntries(paired)

	if len(pending) == 0 && len(paired) == 0 {
		outWarnf("No pairing entries returned by gateway")
	}
	return nil
}

func runDevicesDecision(action string, args []string) error {
	jsonOut, rest, err := parseJSONFlag(args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: openclawctl devices %s <requestId> [--json]", action)
	}

	requestID := rest[0]
	if err := expandFlagEnv("<requestId>", &requestID); err != nil {
		return err
	}

	manifest, err := loadManifest()
	if err != nil {
		return err
	}
	containerName := currentContainerName(manifest)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pending, paired, listUsedFallback, listErr := fetchPairingEntries(ctx, containerName)
	if listErr != nil {
		outWarnf("Preflight pairing lookup failed; proceeding directly: %v", listErr)
	} else {
		if mapped := findPendingRequestIDByDeviceID(pending, requestID); mapped != "" && mapped != requestID {
			outInfof("Resolved deviceId to pending requestId: %s -> %s", accent(requestID), accent(mapped))
			requestID = mapped
		}

		if isPairedID(paired, requestID) {
			return fmt.Errorf("%s is already paired (deviceId). approve/reject expects a pending requestId from `openclawctl devices list`", requestID)
		}
		if !hasPendingRequestID(pending, requestID) {
			if len(pending) == 0 {
				if listUsedFallback {
					return fmt.Errorf("no pending pairing requests (container fallback)")
				}
				return fmt.Errorf("no pending pairing requests")
			}
			return fmt.Errorf("unknown pending requestId %s; available pending requestIds: %s", requestID, strings.Join(pendingRequestIDs(pending), ", "))
		}
	}

	client := gatewayClient()
	var rpcErr error
	appliedVia := "rpc"
	switch action {
	case "approve":
		rpcErr = client.ApprovePairingRequest(ctx, requestID)
	case "reject":
		rpcErr = client.RejectPairingRequest(ctx, requestID)
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
	if rpcErr != nil {
		if fallbackErr := applyPairingDecisionViaContainer(ctx, containerName, action, requestID); fallbackErr != nil {
			return fmt.Errorf("pairing %s failed: rpc=%v; container-fallback=%w", action, rpcErr, fallbackErr)
		}
		outWarnf("RPC pairing %s unavailable, used container fallback: %v", action, rpcErr)
		appliedVia = "container-fallback"
	}

	if jsonOut {
		payload := map[string]string{
			"action":    action,
			"requestId": requestID,
			"status":    "ok",
			"via":       appliedVia,
		}
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return fmt.Errorf("encode json output: %w", marshalErr)
		}
		fmt.Println(string(encoded))
		return nil
	}

	actionLabel := action + "d"
	switch action {
	case "approve":
		actionLabel = "approved"
	case "reject":
		actionLabel = "rejected"
	}
	if appliedVia == "rpc" {
		outSuccessf("Pairing request %s: %s", accent(actionLabel), accent(requestID))
		return nil
	}
	outSuccessf("Pairing request %s via %s: %s", accent(actionLabel), accent("container-fallback"), accent(requestID))
	return nil
}

func fetchPairingEntries(ctx context.Context, containerName string) (pending []map[string]any, paired []map[string]any, usedFallback bool, err error) {
	result, rpcErr := gatewayClient().ListPairingRequests(ctx)
	if rpcErr == nil {
		pending, paired = splitPairingEntries(result)
		return pending, paired, false, nil
	}

	fallbackResult, fallbackErr := listPairingRequestsViaContainer(ctx, containerName)
	if fallbackErr != nil {
		return nil, nil, false, fmt.Errorf("rpc=%v; container-fallback=%w", rpcErr, fallbackErr)
	}

	pending, paired = splitPairingEntries(fallbackResult)
	return pending, paired, true, nil
}

func hasPendingRequestID(pending []map[string]any, requestID string) bool {
	needle := strings.TrimSpace(requestID)
	if needle == "" {
		return false
	}
	for _, entry := range pending {
		candidate := strings.TrimSpace(firstStringField(entry, "requestId", "id"))
		if candidate == needle {
			return true
		}
	}
	return false
}

func findPendingRequestIDByDeviceID(pending []map[string]any, deviceID string) string {
	needle := strings.TrimSpace(deviceID)
	if needle == "" {
		return ""
	}
	for _, entry := range pending {
		candidateDevice := strings.TrimSpace(firstStringField(entry, "deviceId"))
		if candidateDevice != needle {
			continue
		}
		return strings.TrimSpace(firstStringField(entry, "requestId", "id"))
	}
	return ""
}

func isPairedID(paired []map[string]any, id string) bool {
	needle := strings.TrimSpace(id)
	if needle == "" {
		return false
	}
	for _, entry := range paired {
		candidate := strings.TrimSpace(firstStringField(entry, "deviceId", "id"))
		if candidate == needle {
			return true
		}
	}
	return false
}

func pendingRequestIDs(pending []map[string]any) []string {
	ids := make([]string, 0, len(pending))
	for _, entry := range pending {
		requestID := strings.TrimSpace(firstStringField(entry, "requestId", "id"))
		if requestID == "" {
			continue
		}
		ids = append(ids, requestID)
	}
	if len(ids) == 0 {
		return []string{"<none>"}
	}
	return ids
}

func listPairingRequestsViaContainer(ctx context.Context, containerName string) (any, error) {
	args := []string{
		"exec", containerName,
		"node", "openclaw.mjs",
		"devices", "list", "--json",
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}

	var parsed any
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("decode devices list json: %w", err)
	}
	return parsed, nil
}

func applyPairingDecisionViaContainer(ctx context.Context, containerName, action, requestID string) error {
	args := []string{
		"exec", containerName,
		"node", "openclaw.mjs",
		"devices", action, requestID,
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func parseJSONFlag(args []string) (jsonOut bool, rest []string, err error) {
	rest = make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case arg == "--json":
			jsonOut = true
		case strings.HasPrefix(arg, "--json="):
			value := strings.TrimSpace(strings.TrimPrefix(arg, "--json="))
			parsed, parseErr := strconv.ParseBool(value)
			if parseErr != nil {
				return false, nil, fmt.Errorf("invalid --json value %q", value)
			}
			jsonOut = parsed
		case strings.HasPrefix(arg, "-"):
			return false, nil, fmt.Errorf("unknown flag %q", arg)
		default:
			rest = append(rest, arg)
		}
	}
	return jsonOut, rest, nil
}

func printPairingEntries(entries []map[string]any) {
	for _, entry := range entries {
		requestID := firstStringField(entry, "requestId", "id", "deviceId", "clientId")
		if requestID == "" {
			requestID = "-"
		}
		name := firstStringField(entry, "deviceName", "name", "displayName", "clientName")
		deviceID := firstStringField(entry, "deviceId", "clientId")
		platform := firstStringField(entry, "platform", "os", "source")
		ts := firstStringField(entry, "requestedAt", "createdAt", "pairedAt", "approvedAt")

		var details []string
		if name != "" {
			details = append(details, "name="+name)
		}
		if deviceID != "" {
			details = append(details, "deviceId="+deviceID)
		}
		if platform != "" {
			details = append(details, "platform="+platform)
		}
		if ts != "" {
			details = append(details, "at="+ts)
		}

		if len(details) == 0 {
			fmt.Printf("  - %s\n", accent(requestID))
			continue
		}
		fmt.Printf("  - %s (%s)\n", accent(requestID), strings.Join(details, ", "))
	}
}

func splitPairingEntries(raw any) (pending []map[string]any, paired []map[string]any) {
	root, ok := raw.(map[string]any)
	if !ok {
		if direct := toMapSlice(raw); len(direct) > 0 {
			return direct, nil
		}
		return nil, nil
	}

	pending = firstMapSlice(root, "pending", "pendingRequests", "requests", "pairingRequests")
	paired = firstMapSlice(root, "paired", "pairedDevices", "devices", "approved")

	if len(pending) == 0 && len(paired) == 0 {
		items := firstMapSlice(root, "items", "entries", "results")
		for _, item := range items {
			status := strings.ToLower(firstStringField(item, "status", "state"))
			switch {
			case strings.Contains(status, "pending"), strings.Contains(status, "request"):
				pending = append(pending, item)
			case status != "":
				paired = append(paired, item)
			}
		}
		if len(pending) == 0 && len(paired) == 0 {
			pending = items
		}
	}

	return pending, paired
}

func firstMapSlice(root map[string]any, keys ...string) []map[string]any {
	for _, key := range keys {
		value, ok := lookupMapValue(root, key)
		if !ok {
			continue
		}
		items := toMapSlice(value)
		if len(items) > 0 {
			return items
		}
	}
	return nil
}

func toMapSlice(raw any) []map[string]any {
	switch typed := raw.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			parsed, ok := item.(map[string]any)
			if ok {
				result = append(result, parsed)
			}
		}
		return result
	case map[string]any:
		return []map[string]any{typed}
	default:
		return nil
	}
}

func firstStringField(root map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := lookupMapValue(root, key)
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed != "" {
				return trimmed
			}
		case fmt.Stringer:
			trimmed := strings.TrimSpace(typed.String())
			if trimmed != "" {
				return trimmed
			}
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		case float32:
			return strconv.FormatFloat(float64(typed), 'f', -1, 32)
		case int:
			return strconv.Itoa(typed)
		case int64:
			return strconv.FormatInt(typed, 10)
		case uint64:
			return strconv.FormatUint(typed, 10)
		case bool:
			return strconv.FormatBool(typed)
		}
	}
	return ""
}

func lookupMapValue(root map[string]any, key string) (any, bool) {
	if value, ok := root[key]; ok {
		return value, true
	}
	needle := strings.ToLower(strings.TrimSpace(key))
	for currentKey, value := range root {
		if strings.ToLower(strings.TrimSpace(currentKey)) == needle {
			return value, true
		}
	}
	return nil, false
}
