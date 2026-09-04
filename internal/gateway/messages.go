package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"routerprobe/internal/protocol"
	"routerprobe/internal/task"
)

type registerMessage struct {
	DeviceID     string
	Serial       string
	Model        string
	Firmware     string
	ProbeVersion string
	Hostname     string
	Arch         string
	Kernel       string
	Libc         string
	BootID       string
	Capabilities []string
}

func decodeObject(payload []byte) (map[string]json.RawMessage, error) {
	if !utf8.Valid(payload) {
		return nil, errors.New("payload is not valid UTF-8")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil {
		return nil, fmt.Errorf("payload must be a JSON object: %w", err)
	}
	if object == nil {
		return nil, errors.New("payload must be a JSON object")
	}
	return object, nil
}

func requiredString(object map[string]json.RawMessage, name string, minBytes, maxBytes int, ascii bool) (string, error) {
	raw, ok := object[name]
	if !ok {
		return "", fmt.Errorf("%s is required", name)
	}
	return parseString(raw, name, minBytes, maxBytes, ascii)
}

func optionalString(object map[string]json.RawMessage, name string, maxBytes int, ascii bool) (string, error) {
	raw, ok := object[name]
	if !ok {
		return "", nil
	}
	return parseString(raw, name, 0, maxBytes, ascii)
}

func parseString(raw json.RawMessage, name string, minBytes, maxBytes int, ascii bool) (string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", fmt.Errorf("%s must not be null", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%s must be a string", name)
	}
	length := len([]byte(value))
	if length < minBytes || length > maxBytes {
		return "", fmt.Errorf("%s must be %d-%d bytes", name, minBytes, maxBytes)
	}
	if ascii && !isASCII(value) {
		return "", fmt.Errorf("%s must be ASCII", name)
	}
	return value, nil
}

func isASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] > 0x7f {
			return false
		}
	}
	return true
}

func parseRegister(payload []byte) (registerMessage, error) {
	object, err := decodeObject(payload)
	if err != nil {
		return registerMessage{}, err
	}
	var message registerMessage
	if message.DeviceID, err = requiredString(object, "device_id", 1, 128, false); err != nil {
		return registerMessage{}, err
	}
	if message.ProbeVersion, err = requiredString(object, "probe_version", 1, 64, false); err != nil {
		return registerMessage{}, err
	}
	if message.Arch, err = requiredString(object, "arch", 1, 32, true); err != nil {
		return registerMessage{}, err
	}
	if message.BootID, err = requiredString(object, "boot_id", 1, 128, true); err != nil {
		return registerMessage{}, err
	}
	if message.Serial, err = optionalString(object, "serial", 128, false); err != nil {
		return registerMessage{}, err
	}
	if message.Model, err = optionalString(object, "model", 128, false); err != nil {
		return registerMessage{}, err
	}
	if message.Firmware, err = optionalString(object, "firmware", 128, false); err != nil {
		return registerMessage{}, err
	}
	if message.Hostname, err = optionalString(object, "hostname", 255, false); err != nil {
		return registerMessage{}, err
	}
	if message.Kernel, err = optionalString(object, "kernel", 128, false); err != nil {
		return registerMessage{}, err
	}
	if message.Libc, err = optionalString(object, "libc", 64, true); err != nil {
		return registerMessage{}, err
	}

	rawCapabilities, ok := object["capabilities"]
	if !ok {
		return registerMessage{}, errors.New("capabilities is required")
	}
	if bytes.Equal(bytes.TrimSpace(rawCapabilities), []byte("null")) {
		return registerMessage{}, errors.New("capabilities must not be null")
	}
	if err := json.Unmarshal(rawCapabilities, &message.Capabilities); err != nil {
		return registerMessage{}, errors.New("capabilities must be an array of strings")
	}
	if len(message.Capabilities) > 32 {
		return registerMessage{}, errors.New("capabilities must contain at most 32 items")
	}
	for _, capability := range message.Capabilities {
		if len(capability) < 1 || len(capability) > 32 || !isASCII(capability) {
			return registerMessage{}, errors.New("each capability must be 1-32 bytes ASCII")
		}
	}
	return message, nil
}

func parseUnsignedInteger(raw json.RawMessage, name string, maximum uint64) (uint64, error) {
	text := string(bytes.TrimSpace(raw))
	if text == "" || strings.ContainsAny(text, ".eE") || strings.HasPrefix(text, "-") {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil || value > maximum {
		return 0, fmt.Errorf("%s is out of range", name)
	}
	return value, nil
}

func parseNonNegativeNumber(raw json.RawMessage, name string) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value json.Number
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%s must be a number", name)
	}
	number, err := strconv.ParseFloat(value.String(), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 {
		return fmt.Errorf("%s must be a non-negative number", name)
	}
	return nil
}

func validateHeartbeat(payload []byte) error {
	object, err := decodeObject(payload)
	if err != nil {
		return err
	}
	uptime, ok := object["uptime"]
	if !ok {
		return errors.New("uptime is required")
	}
	if _, err := parseUnsignedInteger(uptime, "uptime", math.MaxInt64); err != nil {
		return err
	}
	runningTasks, ok := object["running_tasks"]
	if !ok {
		return errors.New("running_tasks is required")
	}
	if _, err := parseUnsignedInteger(runningTasks, "running_tasks", 65535); err != nil {
		return err
	}
	if load1, ok := object["load1"]; ok {
		if bytes.Equal(bytes.TrimSpace(load1), []byte("null")) {
			return errors.New("load1 must not be null")
		}
		if err := parseNonNegativeNumber(load1, "load1"); err != nil {
			return err
		}
	}
	if freeMemory, ok := object["free_memory"]; ok {
		if bytes.Equal(bytes.TrimSpace(freeMemory), []byte("null")) {
			return errors.New("free_memory must not be null")
		}
		if _, err := parseUnsignedInteger(freeMemory, "free_memory", math.MaxInt64); err != nil {
			return err
		}
	}
	return nil
}

func parseBoolean(raw json.RawMessage, name string) (bool, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, fmt.Errorf("%s must not be null", name)
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return value, nil
}

func parseSignedInteger(raw json.RawMessage, name string, minimum, maximum int64) (int64, error) {
	text := string(bytes.TrimSpace(raw))
	if text == "" || strings.ContainsAny(text, ".eE") {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s is out of range", name)
	}
	return value, nil
}

func parseTaskAck(payload []byte) (task.Ack, error) {
	object, err := decodeObject(payload)
	if err != nil {
		return task.Ack{}, err
	}
	replyRaw, ok := object["reply_to"]
	if !ok {
		return task.Ack{}, errors.New("reply_to is required")
	}
	replyTo, err := parseUnsignedInteger(replyRaw, "reply_to", math.MaxUint64)
	if err != nil || replyTo == 0 {
		if err != nil {
			return task.Ack{}, err
		}
		return task.Ack{}, errors.New("reply_to must not be zero")
	}
	taskID, err := requiredString(object, "task_id", 1, 128, false)
	if err != nil {
		return task.Ack{}, err
	}
	acceptedRaw, ok := object["accepted"]
	if !ok {
		return task.Ack{}, errors.New("accepted is required")
	}
	accepted, err := parseBoolean(acceptedRaw, "accepted")
	if err != nil {
		return task.Ack{}, err
	}
	state, err := requiredString(object, "state", 1, 32, true)
	if err != nil {
		return task.Ack{}, err
	}
	return task.Ack{ReplyTo: replyTo, TaskID: taskID, Accepted: accepted, State: state}, nil
}

func parseTaskResult(payload []byte) (task.Result, error) {
	object, err := decodeObject(payload)
	if err != nil {
		return task.Result{}, err
	}
	taskID, err := requiredString(object, "task_id", 1, 128, false)
	if err != nil {
		return task.Result{}, err
	}
	status, err := requiredString(object, "status", 1, 32, true)
	if err != nil {
		return task.Result{}, err
	}
	if status != "success" && status != "failed" && status != "timeout" {
		return task.Result{}, errors.New("status must be success, failed, or timeout")
	}
	startedRaw, ok := object["started_at"]
	if !ok {
		return task.Result{}, errors.New("started_at is required")
	}
	startedAt, err := parseUnsignedInteger(startedRaw, "started_at", math.MaxInt64)
	if err != nil {
		return task.Result{}, err
	}
	finishedRaw, ok := object["finished_at"]
	if !ok {
		return task.Result{}, errors.New("finished_at is required")
	}
	finishedAt, err := parseUnsignedInteger(finishedRaw, "finished_at", math.MaxInt64)
	if err != nil {
		return task.Result{}, err
	}
	if finishedAt < startedAt {
		return task.Result{}, errors.New("finished_at must not be earlier than started_at")
	}
	exitRaw, ok := object["exit_code"]
	if !ok {
		return task.Result{}, errors.New("exit_code is required")
	}
	exitCode, err := parseSignedInteger(exitRaw, "exit_code", math.MinInt32, math.MaxInt32)
	if err != nil {
		return task.Result{}, err
	}
	stdout, err := requiredString(object, "stdout", 0, int(protocol.MaxControlPayload), false)
	if err != nil {
		return task.Result{}, err
	}
	stderr, err := requiredString(object, "stderr", 0, int(protocol.MaxControlPayload), false)
	if err != nil {
		return task.Result{}, err
	}
	truncatedRaw, ok := object["truncated"]
	if !ok {
		return task.Result{}, errors.New("truncated is required")
	}
	truncated, err := parseBoolean(truncatedRaw, "truncated")
	if err != nil {
		return task.Result{}, err
	}
	resultRaw, ok := object["result"]
	if !ok {
		return task.Result{}, errors.New("result is required")
	}
	if _, err := decodeObject(resultRaw); err != nil {
		return task.Result{}, fmt.Errorf("result must be a JSON object: %w", err)
	}
	return task.Result{
		TaskID: taskID, Status: status, StartedAt: int64(startedAt), FinishedAt: int64(finishedAt),
		ExitCode: int(exitCode), Stdout: stdout, Stderr: stderr, Truncated: truncated, Details: append(json.RawMessage(nil), resultRaw...),
	}, nil
}
