package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// MCPCallTimeout is the maximum time an MCP call may take before aborting.
var MCPCallTimeout = 5 * time.Minute

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Method string `json:"method,omitempty"` // for notifications
}

type MCPClient struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	mu        sync.Mutex
	nextID    int
	tools     []Tool
	pending   map[int]chan *jsonrpcResponse
	pendMu    sync.Mutex
	closed    chan struct{}
	closeOnce sync.Once
}

func NewMCPClient(command string) (*MCPClient, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty MCP server command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("opening MCP stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("opening MCP stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting MCP server: %w", err)
	}

	c := &MCPClient{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  bufio.NewReader(stdout),
		pending: make(map[int]chan *jsonrpcResponse),
		closed:  make(chan struct{}),
	}

	// Start the dedicated reader goroutine for the lifetime of the client
	go c.readLoop()

	if err := c.initialize(); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("MCP initialize: %w", err)
	}

	if err := c.loadTools(); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("MCP tools/list: %w", err)
	}

	return c, nil
}

// readLoop reads JSON-RPC responses from stdout and dispatches them by ID
// to waiting callers. Notifications (no ID) are dropped.
func (c *MCPClient) readLoop() {
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			// EOF or pipe broken — notify all pending callers
			c.pendMu.Lock()
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.pendMu.Unlock()
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var resp jsonrpcResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}
		// Skip notifications
		if resp.ID == 0 && resp.Method != "" {
			continue
		}
		c.pendMu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.pendMu.Unlock()
		if ok {
			ch <- &resp
			close(ch)
		}
	}
}

// call sends a JSON-RPC request and waits for the matching response.
// Times out after MCPCallTimeout. The reader goroutine handles dispatch,
// so timeouts no longer leak goroutines.
func (c *MCPClient) call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling MCP request: %w", err)
	}
	data = append(data, '\n')

	// Register pending response channel before sending
	respCh := make(chan *jsonrpcResponse, 1)
	c.pendMu.Lock()
	c.pending[id] = respCh
	c.pendMu.Unlock()

	c.mu.Lock()
	_, werr := c.stdin.Write(data)
	c.mu.Unlock()
	if werr != nil {
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
		return nil, fmt.Errorf("write to MCP: %w", werr)
	}

	select {
	case resp, ok := <-respCh:
		if !ok || resp == nil {
			return nil, fmt.Errorf("MCP connection closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(MCPCallTimeout):
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
		return nil, fmt.Errorf("MCP call %q timed out after %v", method, MCPCallTimeout)
	}
}

func (c *MCPClient) initialize() error {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    AppName,
			"version": AppVersion,
		},
	}
	_, err := c.call("initialize", params)
	if err != nil {
		return fmt.Errorf("MCP initialize call: %w", err)
	}
	notif := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshaling initialized notification: %w", err)
	}
	data = append(data, '\n')
	c.mu.Lock()
	_, err = c.stdin.Write(data)
	c.mu.Unlock()
	if err != nil {
		return fmt.Errorf("sending initialized notification: %w", err)
	}
	return nil
}

func (c *MCPClient) loadTools() error {
	result, err := c.call("tools/list", nil)
	if err != nil {
		return fmt.Errorf("MCP tools/list call: %w", err)
	}

	var parsed struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return fmt.Errorf("parsing MCP tools/list response: %w", err)
	}

	c.tools = nil
	for _, t := range parsed.Tools {
		c.tools = append(c.tools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	return nil
}

func (c *MCPClient) Tools() []Tool {
	return c.tools
}

func (c *MCPClient) CallTool(name string, args map[string]interface{}) (string, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}
	result, err := c.call("tools/call", params)
	if err != nil {
		return "", fmt.Errorf("MCP tools/call for %q: %w", name, err)
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return string(result), nil
	}

	var parts []string
	for _, ct := range parsed.Content {
		if ct.Type == "text" && ct.Text != "" {
			parts = append(parts, ct.Text)
		}
	}
	output := strings.Join(parts, "\n")

	if parsed.IsError {
		return "", fmt.Errorf("tool error: %s", output)
	}

	if len(output) > MaxToolOutput {
		output = output[:MaxToolOutput] + "\n[... output truncated ...]"
	}
	return output, nil
}

func (c *MCPClient) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		c.stdin.Close()

		if c.cmd == nil {
			return
		}

		done := make(chan error, 1)
		go func() { done <- c.cmd.Wait() }()

		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case <-done:
			return
		case <-timer.C:
			if c.cmd.Process != nil {
				c.cmd.Process.Signal(syscall.SIGTERM)
			}
		}

		timer2 := time.NewTimer(2 * time.Second)
		defer timer2.Stop()
		select {
		case <-done:
		case <-timer2.C:
			if c.cmd.Process != nil {
				c.cmd.Process.Kill()
			}
		}
	})
}
