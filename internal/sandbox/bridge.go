package sandbox

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

//go:embed bridge.py
var bridgeScript []byte

// JSON-RPC 2.0 message types.

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      any    `json:"id,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
// Result must NOT have omitempty — nil results cause bridge hangs.
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

type rawMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      any             `json:"id,omitempty"`
}

// PrimitiveParams is the shape of params for primitive callbacks from the bridge.
type PrimitiveParams struct {
	Args   []any          `json:"args,omitempty"`
	Kwargs map[string]any `json:"kwargs,omitempty"`
}

// PrimitiveHandler handles a primitive callback from the bridge.
type PrimitiveHandler func(args []any, kwargs map[string]any) (any, error)

// Bridge manages the Python bridge subprocess and JSON-RPC communication.
type Bridge struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	reader   *bufio.Reader
	mu       sync.Mutex
	nextID   int
	pending  map[int]chan *Response
	handlers map[string]PrimitiveHandler
	tmpDir   string
	done     chan struct{}
}

// NewBridge starts the Monty sandbox bridge subprocess.
// The embedded bridge.py is written to a temp directory and run via uv.
func NewBridge() (*Bridge, error) {
	tmpDir, err := os.MkdirTemp("", "cleared-bridge-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	bridgePath := filepath.Join(tmpDir, "bridge.py")
	if err := os.WriteFile(bridgePath, bridgeScript, 0o644); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("writing bridge.py: %w", err)
	}

	cmd := exec.Command("uv", "run", "--with", "pydantic-monty", "--no-project", "python3", bridgePath)
	cmd.Dir = tmpDir
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("start bridge: %w", err)
	}

	b := &Bridge{
		cmd:      cmd,
		stdin:    stdin,
		reader:   bufio.NewReader(stdout),
		pending:  make(map[int]chan *Response),
		handlers: make(map[string]PrimitiveHandler),
		tmpDir:   tmpDir,
		done:     make(chan struct{}),
	}
	go b.readLoop()
	return b, nil
}

// RegisterPrimitive registers a handler for a named primitive.
func (b *Bridge) RegisterPrimitive(name string, handler PrimitiveHandler) {
	b.handlers[name] = handler
}

// PrimitiveNames returns the names of all registered primitives.
func (b *Bridge) PrimitiveNames() []string {
	names := make([]string, 0, len(b.handlers))
	for name := range b.handlers {
		names = append(names, name)
	}
	return names
}

// RunScript sends a script to the bridge for execution. The externals list
// declares which primitive functions the script may call. Times out after 30s.
func (b *Bridge) RunScript(script string, externals []string) (any, error) {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	ch := make(chan *Response, 1)
	b.pending[id] = ch
	b.mu.Unlock()

	if err := b.send(Request{
		JSONRPC: "2.0",
		Method:  "run",
		Params:  map[string]any{"script": script, "external_functions": externals},
		ID:      id,
	}); err != nil {
		return nil, err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("%s", resp.Error.Message)
		}
		return resp.Result, nil
	case <-b.done:
		return nil, errors.New("bridge process exited unexpectedly")
	case <-time.After(30 * time.Second):
		return nil, errors.New("script execution timed out after 30s")
	}
}

// Shutdown sends the shutdown notification and cleans up.
func (b *Bridge) Shutdown() error {
	_ = b.send(Request{JSONRPC: "2.0", Method: "shutdown"})
	err := b.cmd.Wait()
	os.RemoveAll(b.tmpDir)
	return err
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
	defer close(b.done)
	for {
		line, err := b.reader.ReadString('\n')
		if err != nil {
			return
		}

		var msg rawMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		// Response to one of our outgoing requests.
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

		// Primitive callback from the bridge.
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
			Error:   &RPCError{Code: -32601, Message: "unknown primitive: " + msg.Method},
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

	_ = b.send(Response{JSONRPC: "2.0", Result: result, ID: msg.ID})
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
