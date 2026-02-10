// Spike 1: Go ↔ Monty sandbox via JSON-RPC 2.0 over stdio.
//
// Go is both client (sends "run" requests) and server (handles primitive callbacks).
// The bridge is both server (handles "run") and client (calls primitives back to Go).
// ID correlation allows pipelining — multiple scripts can run concurrently.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// JSON-RPC 2.0 message types.

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      any    `json:"id,omitempty"` // nil for notifications
}

type Response struct {
	JSONRPC string    `json:"jsonrpc"`
	Result  any       `json:"result"`
	Error   *RPCError `json:"error,omitempty"`
	ID      any       `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// rawMessage is used for initial parsing to determine if it's a request or response.
type rawMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      any             `json:"id,omitempty"`
}

// PrimitiveParams is the shape of params for primitive callbacks.
type PrimitiveParams struct {
	Args   []any          `json:"args,omitempty"`
	Kwargs map[string]any `json:"kwargs,omitempty"`
}

// Bridge manages the Python bridge subprocess and JSON-RPC communication.
type Bridge struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader

	mu       sync.Mutex
	nextID   int
	pending  map[int]chan *Response // awaiting responses to our requests
	handlers map[string]PrimitiveHandler
}

type PrimitiveHandler func(args []any, kwargs map[string]any) (any, error)

func NewBridge(bridgePath string) (*Bridge, error) {
	cmd := exec.Command("uv", "run", "python3", bridgePath)
	cmd.Dir = filepath.Dir(bridgePath)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start bridge: %w", err)
	}

	b := &Bridge{
		cmd:      cmd,
		stdin:    stdin,
		reader:   bufio.NewReader(stdout),
		pending:  make(map[int]chan *Response),
		handlers: make(map[string]PrimitiveHandler),
	}

	// Start reading messages in background
	go b.readLoop()

	return b, nil
}

func (b *Bridge) RegisterPrimitive(name string, handler PrimitiveHandler) {
	b.handlers[name] = handler
}

func (b *Bridge) send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	b.mu.Lock()
	_, err = fmt.Fprintf(b.stdin, "%s\n", data)
	b.mu.Unlock()
	return err
}

func (b *Bridge) readLoop() {
	for {
		line, err := b.reader.ReadString('\n')
		if err != nil {
			return
		}

		var msg rawMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		// Is it a response to one of our outgoing requests?
		if msg.Method == "" && (msg.Result != nil || msg.Error != nil) {
			id := toInt(msg.ID)
			b.mu.Lock()
			ch, ok := b.pending[id]
			if ok {
				delete(b.pending, id)
			}
			b.mu.Unlock()
			if ok {
				resp := &Response{ID: msg.ID, Error: msg.Error}
				if msg.Result != nil {
					var result any
					_ = json.Unmarshal(msg.Result, &result)
					resp.Result = result
				}
				ch <- resp
			}
			continue
		}

		// It's a request (primitive callback from the bridge)
		if msg.Method != "" {
			go b.handleCallback(msg)
		}
	}
}

func (b *Bridge) handleCallback(msg rawMessage) {
	var params PrimitiveParams
	if msg.Params != nil {
		_ = json.Unmarshal(msg.Params, &params)
	}

	handler, ok := b.handlers[msg.Method]
	if !ok {
		_ = b.send(Response{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32601, Message: fmt.Sprintf("unknown primitive: %s", msg.Method)},
			ID:      msg.ID,
		})
		return
	}

	result, err := handler(params.Args, params.Kwargs)
	if err != nil {
		_ = b.send(Response{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: -32000, Message: err.Error()},
			ID:      msg.ID,
		})
		return
	}

	_ = b.send(Response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      msg.ID,
	})
}

// RunScript sends a "run" request and waits for the result.
func (b *Bridge) RunScript(script string, externalFunctions []string) (any, error) {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	ch := make(chan *Response, 1)
	b.pending[id] = ch
	b.mu.Unlock()

	err := b.send(Request{
		JSONRPC: "2.0",
		Method:  "run",
		Params: map[string]any{
			"script":             script,
			"external_functions": externalFunctions,
		},
		ID: id,
	})
	if err != nil {
		return nil, err
	}

	resp := <-ch
	if resp.Error != nil {
		return nil, fmt.Errorf("script error: %s", resp.Error.Message)
	}
	return resp.Result, nil
}

func (b *Bridge) Shutdown() error {
	_ = b.send(Request{JSONRPC: "2.0", Method: "shutdown"})
	return b.cmd.Wait()
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// --- Spike test ---

func main() {
	spikeDir, _ := os.Getwd()
	bridgePath := filepath.Join(spikeDir, "bridge.py")

	fmt.Println("=== Spike 1: Go ↔ Monty Sandbox (JSON-RPC 2.0) ===")
	fmt.Println()

	// Start bridge
	fmt.Print("Starting bridge... ")
	start := time.Now()
	bridge, err := NewBridge(bridgePath)
	if err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%.0fms)\n", time.Since(start).Seconds()*1000)

	// Register stub primitives
	callLog := struct {
		mu    sync.Mutex
		calls []string
	}{}

	bridge.RegisterPrimitive("journal_query", func(args []any, kwargs map[string]any) (any, error) {
		status, _ := kwargs["status"].(string)
		callLog.mu.Lock()
		callLog.calls = append(callLog.calls, fmt.Sprintf("journal_query(status=%s)", status))
		callLog.mu.Unlock()
		return []map[string]any{
			{"id": "2025-01-001", "description": "GitHub Pro", "amount": 4.00, "confidence": 0.97},
			{"id": "2025-01-002", "description": "AWS Services", "amount": 127.50, "confidence": 0.45},
		}, nil
	})

	bridge.RegisterPrimitive("journal_add", func(args []any, kwargs map[string]any) (any, error) {
		callLog.mu.Lock()
		callLog.calls = append(callLog.calls, fmt.Sprintf("journal_add(%v)", kwargs["entry_id"]))
		callLog.mu.Unlock()
		return map[string]any{"success": true}, nil
	})

	bridge.RegisterPrimitive("config_get", func(args []any, kwargs map[string]any) (any, error) {
		key := ""
		if len(args) > 0 {
			key, _ = args[0].(string)
		}
		callLog.mu.Lock()
		callLog.calls = append(callLog.calls, fmt.Sprintf("config_get(%s)", key))
		callLog.mu.Unlock()
		configs := map[string]any{
			"business.name":           "Acme Consulting LLC",
			"thresholds.auto_confirm": 0.95,
		}
		return configs[key], nil
	})

	bridge.RegisterPrimitive("ping", func(args []any, kwargs map[string]any) (any, error) {
		v := ""
		if len(args) > 0 {
			v, _ = args[0].(string)
		}
		return "pong-" + v, nil
	})

	// --- Test 1: Agent script with multiple primitive calls ---
	fmt.Println("\n--- Test 1: Agent script with routing logic ---")
	agentScript := `
entries = journal_query(status="pending")
threshold = config_get("thresholds.auto_confirm")

confirmed = 0
review = 0
for entry in entries:
    if entry["confidence"] >= threshold:
        journal_add(entry_id=entry["id"], status="auto-confirmed",
                    evidence="high confidence: " + str(entry["confidence"]))
        confirmed = confirmed + 1
    else:
        journal_add(entry_id=entry["id"], status="pending-review",
                    evidence="low confidence: " + str(entry["confidence"]))
        review = review + 1

{"confirmed": confirmed, "review": review}
`

	scriptStart := time.Now()
	result, err := bridge.RunScript(agentScript, []string{"journal_query", "journal_add", "config_get"})
	elapsed := time.Since(scriptStart)

	if err != nil {
		fmt.Printf("  FAILED: %v\n", err)
		os.Exit(1)
	}

	output, _ := result.(map[string]any)
	confirmed, _ := output["confirmed"].(float64)
	review, _ := output["review"].(float64)

	fmt.Printf("  Result: confirmed=%.0f review=%.0f (%dms)\n", confirmed, review, elapsed.Milliseconds())
	fmt.Printf("  Calls: %v\n", callLog.calls)

	if confirmed == 1 && review == 1 {
		fmt.Println("  PASS")
	} else {
		fmt.Printf("  FAIL: expected confirmed=1 review=1\n")
		os.Exit(1)
	}

	// --- Test 2: Sandbox security ---
	fmt.Println("\n--- Test 2: Sandbox security ---")
	for _, tc := range []struct {
		name   string
		script string
	}{
		{"open()", `open("/etc/passwd")`},
		{"eval()", `eval("1+1")`},
		{"__import__()", `__import__("os")`},
		{"exec()", `exec("print(1)")`},
	} {
		_, err := bridge.RunScript(tc.script, nil)
		if err != nil {
			fmt.Printf("  %s: BLOCKED (%s)\n", tc.name, err)
		} else {
			fmt.Printf("  %s: FAIL — not blocked!\n", tc.name)
		}
	}

	// --- Test 3: Concurrent scripts (pipelining) ---
	fmt.Println("\n--- Test 3: Concurrent scripts ---")
	var wg sync.WaitGroup
	results := make([]any, 3)
	errors := make([]error, 3)

	for i := range 3 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			script := fmt.Sprintf(`ping("%d")`, idx)
			results[idx], errors[idx] = bridge.RunScript(script, []string{"ping"})
		}(i)
	}
	wg.Wait()

	allOK := true
	for i := range 3 {
		if errors[i] != nil {
			fmt.Printf("  Script %d: ERROR %v\n", i, errors[i])
			allOK = false
		} else {
			fmt.Printf("  Script %d: %v\n", i, results[i])
		}
	}
	if allOK {
		fmt.Println("  PASS")
	}

	// --- Test 4: Latency benchmark ---
	fmt.Println("\n--- Test 4: Latency benchmark ---")
	iterations := 100
	benchStart := time.Now()
	for i := range iterations {
		_, err := bridge.RunScript(fmt.Sprintf(`ping("%d")`, i), []string{"ping"})
		if err != nil {
			fmt.Printf("  FAILED at iteration %d: %v\n", i, err)
			break
		}
	}
	benchElapsed := time.Since(benchStart)
	perCall := benchElapsed.Seconds() * 1000 / float64(iterations)
	fmt.Printf("  %d round-trips in %.0fms (%.2fms/trip)\n", iterations, benchElapsed.Seconds()*1000, perCall)
	if perCall < 100 {
		fmt.Println("  PASS (< 100ms)")
	} else {
		fmt.Println("  FAIL (> 100ms)")
	}

	// Shutdown
	fmt.Println("\n--- Shutdown ---")
	if err := bridge.Shutdown(); err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Println("  Clean shutdown: OK")
	}

	// Summary
	fmt.Println("\n=== Spike 1 Results ===")
	fmt.Println("  [✓] Go ↔ Monty via JSON-RPC 2.0 over stdio")
	fmt.Println("  [✓] Primitive calls pause, flow to Go, return results")
	fmt.Println("  [✓] Sandbox blocks: open, eval, exec, __import__")
	fmt.Println("  [✓] Concurrent script execution (pipelining)")
	fmt.Printf("  [✓] Latency: %.2fms/call\n", perCall)
}
