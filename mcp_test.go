package main

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestMCPClientCall(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	client := &MCPClient{
		stdin:   stdinW,
		stdout:  bufio.NewReader(stdoutR),
		pending: make(map[int]chan *jsonrpcResponse),
		closed:  make(chan struct{}),
	}

	go client.readLoop()

	// Simulate MCP server
	go func() {
		defer stdoutW.Close()
		scanner := bufio.NewScanner(stdinR)
		for scanner.Scan() {
			var req jsonrpcRequest
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				continue
			}
			var result interface{}
			switch req.Method {
			case "initialize":
				result = map[string]interface{}{"protocolVersion": "2024-11-05"}
			case "tools/list":
				result = map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "test_tool",
							"description": "A test tool",
							"inputSchema": map[string]interface{}{
								"type": "object",
							},
						},
					},
				}
			case "tools/call":
				result = map[string]interface{}{
					"content": []map[string]string{
						{"type": "text", "text": "tool result"},
					},
				}
			default:
				result = map[string]interface{}{}
			}
			resp := jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mustMarshal(result),
			}
			data, _ := json.Marshal(resp)
			stdoutW.Write(append(data, '\n'))
		}
	}()

	// Test initialize
	if err := client.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	// Test loadTools
	if err := client.loadTools(); err != nil {
		t.Fatalf("loadTools: %v", err)
	}
	tools := client.Tools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}
	if len(tools) > 0 && tools[0].Function.Name != "test_tool" {
		t.Errorf("tool name = %q", tools[0].Function.Name)
	}

	// Test CallTool
	output, err := client.CallTool("test_tool", map[string]interface{}{"key": "val"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if output != "tool result" {
		t.Errorf("output = %q", output)
	}

	client.Close()
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}

func TestMCPClientCallTimeout(t *testing.T) {
	origTimeout := MCPCallTimeout
	MCPCallTimeout = 50 * time.Millisecond
	defer func() { MCPCallTimeout = origTimeout }()

	stdinR, stdinW := io.Pipe()
	// Drain stdin so Write doesn't block (io.Pipe is unbuffered)
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := stdinR.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	client := &MCPClient{
		stdin:   stdinW,
		pending: make(map[int]chan *jsonrpcResponse),
		closed:  make(chan struct{}),
	}

	_, err := client.call("slow/method", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, want timeout", err.Error())
	}

	stdinW.Close()
}

func TestMCPClientClose(t *testing.T) {
	_, stdinW := io.Pipe()

	client := &MCPClient{
		stdin:   stdinW,
		pending: make(map[int]chan *jsonrpcResponse),
		closed:  make(chan struct{}),
	}

	client.Close()

	_, err := stdinW.Write([]byte("x"))
	if err == nil {
		t.Error("expected write to closed stdin to fail")
	}
}

func TestMCPClientCloseIdempotent(t *testing.T) {
	_, stdinW := io.Pipe()

	client := &MCPClient{
		stdin:   stdinW,
		pending: make(map[int]chan *jsonrpcResponse),
		closed:  make(chan struct{}),
	}

	client.Close()
	client.Close() // should not panic
}

func TestMCPClientToolError(t *testing.T) {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()

	client := &MCPClient{
		stdin:   stdinW,
		stdout:  bufio.NewReader(stdoutR),
		pending: make(map[int]chan *jsonrpcResponse),
		closed:  make(chan struct{}),
	}
	go client.readLoop()

	go func() {
		defer stdoutW.Close()
		scanner := bufio.NewScanner(stdinR)
		for scanner.Scan() {
			var req jsonrpcRequest
			json.Unmarshal(scanner.Bytes(), &req)
			resp := jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mustMarshal(map[string]interface{}{
					"content": []map[string]string{{"type": "text", "text": "something broke"}},
					"isError": true,
				}),
			}
			data, _ := json.Marshal(resp)
			stdoutW.Write(append(data, '\n'))
		}
	}()

	_, err := client.CallTool("bad_tool", nil)
	if err == nil {
		t.Fatal("expected tool error")
	}
	if !strings.Contains(err.Error(), "tool error") {
		t.Errorf("error = %q", err.Error())
	}

	client.Close()
}
