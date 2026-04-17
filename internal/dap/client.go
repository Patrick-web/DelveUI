package dap

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-dap"
)

// Client is a minimal DAP client: one in-flight request seq counter,
// a map of seq → response waiter, and a fan-out channel for events.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	writeM sync.Mutex
	seq    int64

	mu      sync.Mutex
	pending map[int]chan dap.Message

	events chan dap.Message
	done   chan struct{}
	closed atomic.Bool
}

func Dial(ctx context.Context, addr string) (*Client, error) {
	var d net.Dialer
	// Retry briefly — dlv dap takes a moment to listen.
	deadline := time.Now().Add(8 * time.Second)
	var conn net.Conn
	var err error
	for time.Now().Before(deadline) {
		conn, err = d.DialContext(ctx, "tcp", addr)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	if err != nil {
		return nil, fmt.Errorf("dial dlv dap %s: %w", addr, err)
	}
	c := &Client{
		conn:    conn,
		reader:  bufio.NewReader(conn),
		pending: make(map[int]chan dap.Message),
		events:  make(chan dap.Message, 64),
		done:    make(chan struct{}),
	}
	go c.readLoop()
	return c, nil
}

func (c *Client) Events() <-chan dap.Message { return c.events }
func (c *Client) Done() <-chan struct{}      { return c.done }

func (c *Client) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	err := c.conn.Close()
	close(c.done)
	return err
}

func (c *Client) nextSeq() int { return int(atomic.AddInt64(&c.seq, 1)) }

// Send issues a request and waits for its matching response.
// `req` must be a concrete *dap.XxxRequest with Command and Type set by the caller.
func (c *Client) Send(req dap.RequestMessage) (dap.Message, error) {
	if c.closed.Load() {
		return nil, io.ErrClosedPipe
	}
	seq := c.nextSeq()
	// set envelope
	pm := req.GetRequest()
	pm.Seq = seq
	pm.Type = "request"

	ch := make(chan dap.Message, 1)
	c.mu.Lock()
	c.pending[seq] = ch
	c.mu.Unlock()

	c.writeM.Lock()
	err := dap.WriteProtocolMessage(c.conn, req)
	c.writeM.Unlock()
	if err != nil {
		c.mu.Lock()
		delete(c.pending, seq)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-c.done:
		return nil, io.EOF
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("dap request %q timed out", pm.Command)
	}
}

func (c *Client) readLoop() {
	defer c.Close()
	defer close(c.events)
	for {
		msg, err := dap.ReadProtocolMessage(c.reader)
		if err != nil {
			return
		}
		switch m := msg.(type) {
		case dap.ResponseMessage:
			r := m.GetResponse()
			c.mu.Lock()
			ch, ok := c.pending[r.RequestSeq]
			delete(c.pending, r.RequestSeq)
			c.mu.Unlock()
			if ok {
				ch <- msg
			}
		case dap.EventMessage:
			select {
			case c.events <- msg:
			default:
				// drop if consumer slow; DAP events are lossy-tolerable for UI
			}
		default:
			_ = m
		}
	}
}

// checkError returns an error if the DAP response is an ErrorResponse.
func checkError(resp dap.Message) error {
	if er, ok := resp.(*dap.ErrorResponse); ok {
		detail := er.Message
		if er.Body.Error != nil && er.Body.Error.Format != "" {
			detail = er.Body.Error.Format
			for k, v := range er.Body.Error.Variables {
				detail = strings.ReplaceAll(detail, "{"+k+"}", v)
			}
		}
		return fmt.Errorf("DAP error: %s", detail)
	}
	return nil
}

// Helpers wrap common requests with simpler APIs.

func (c *Client) Initialize(clientID string) (*dap.InitializeResponse, error) {
	req := &dap.InitializeRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "initialize"},
		Arguments: dap.InitializeRequestArguments{ClientID: clientID, ClientName: "DelveUI", AdapterID: "delve", Locale: "en", LinesStartAt1: true, ColumnsStartAt1: true, PathFormat: "path", SupportsRunInTerminalRequest: false},
	}
	resp, err := c.Send(req)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil { return nil, err }
	r, ok := resp.(*dap.InitializeResponse); if !ok { return nil, fmt.Errorf("unexpected response type") }; return r, nil
}

// Launch sends a raw launch with the given args map (Delve-specific keys like program/cwd/env).
func (c *Client) Launch(args map[string]any) error {
	body, _ := json.Marshal(args)
	req := &dap.LaunchRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "launch"},
		Arguments: body,
	}
	resp, err := c.Send(req)
	if err != nil {
		return err
	}
	if er, ok := resp.(*dap.ErrorResponse); ok {
		detail := er.Message
		if er.Body.Error != nil {
			if er.Body.Error.Format != "" {
				detail = er.Body.Error.Format
			}
			// substitute {variables} from Variables map
			for k, v := range er.Body.Error.Variables {
				detail = strings.ReplaceAll(detail, "{"+k+"}", v)
			}
		}
		return fmt.Errorf("launch failed: %s (args: %s)", detail, string(body))
	}
	return nil
}

func (c *Client) ConfigurationDone() error {
	req := &dap.ConfigurationDoneRequest{
		Request: dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "configurationDone"},
	}
	_, err := c.Send(req)
	return err
}

func (c *Client) SetBreakpoints(source string, lines []int) (*dap.SetBreakpointsResponse, error) {
	bps := make([]dap.SourceBreakpoint, len(lines))
	for i, l := range lines {
		bps[i] = dap.SourceBreakpoint{Line: l}
	}
	req := &dap.SetBreakpointsRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "setBreakpoints"},
		Arguments: dap.SetBreakpointsArguments{Source: dap.Source{Path: source}, Breakpoints: bps},
	}
	resp, err := c.Send(req)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil { return nil, err }
	r, ok := resp.(*dap.SetBreakpointsResponse); if !ok { return nil, fmt.Errorf("unexpected response type") }; return r, nil
}

func (c *Client) Continue(threadID int) error {
	req := &dap.ContinueRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "continue"},
		Arguments: dap.ContinueArguments{ThreadId: threadID},
	}
	_, err := c.Send(req)
	return err
}

func (c *Client) Next(threadID int) error {
	req := &dap.NextRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "next"},
		Arguments: dap.NextArguments{ThreadId: threadID},
	}
	_, err := c.Send(req)
	return err
}

func (c *Client) StepIn(threadID int) error {
	req := &dap.StepInRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "stepIn"},
		Arguments: dap.StepInArguments{ThreadId: threadID},
	}
	_, err := c.Send(req)
	return err
}

func (c *Client) StepOut(threadID int) error {
	req := &dap.StepOutRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "stepOut"},
		Arguments: dap.StepOutArguments{ThreadId: threadID},
	}
	_, err := c.Send(req)
	return err
}

func (c *Client) Pause(threadID int) error {
	req := &dap.PauseRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "pause"},
		Arguments: dap.PauseArguments{ThreadId: threadID},
	}
	_, err := c.Send(req)
	return err
}

func (c *Client) Threads() (*dap.ThreadsResponse, error) {
	req := &dap.ThreadsRequest{
		Request: dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "threads"},
	}
	resp, err := c.Send(req)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil { return nil, err }
	r, ok := resp.(*dap.ThreadsResponse); if !ok { return nil, fmt.Errorf("unexpected response type") }; return r, nil
}

func (c *Client) StackTrace(threadID int) (*dap.StackTraceResponse, error) {
	req := &dap.StackTraceRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "stackTrace"},
		Arguments: dap.StackTraceArguments{ThreadId: threadID, Levels: 50},
	}
	resp, err := c.Send(req)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil { return nil, err }
	r, ok := resp.(*dap.StackTraceResponse); if !ok { return nil, fmt.Errorf("unexpected response type") }; return r, nil
}

func (c *Client) Scopes(frameID int) (*dap.ScopesResponse, error) {
	req := &dap.ScopesRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "scopes"},
		Arguments: dap.ScopesArguments{FrameId: frameID},
	}
	resp, err := c.Send(req)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil { return nil, err }
	r, ok := resp.(*dap.ScopesResponse); if !ok { return nil, fmt.Errorf("unexpected response type") }; return r, nil
}

func (c *Client) Variables(ref int) (*dap.VariablesResponse, error) {
	req := &dap.VariablesRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "variables"},
		Arguments: dap.VariablesArguments{VariablesReference: ref},
	}
	resp, err := c.Send(req)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil { return nil, err }
	r, ok := resp.(*dap.VariablesResponse); if !ok { return nil, fmt.Errorf("unexpected response type") }; return r, nil
}

func (c *Client) Evaluate(expr string, frameID int) (*dap.EvaluateResponse, error) {
	req := &dap.EvaluateRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "evaluate"},
		Arguments: dap.EvaluateArguments{Expression: expr, FrameId: frameID, Context: "repl"},
	}
	resp, err := c.Send(req)
	if err != nil {
		return nil, err
	}
	if err := checkError(resp); err != nil { return nil, err }
	r, ok := resp.(*dap.EvaluateResponse); if !ok { return nil, fmt.Errorf("unexpected response type") }; return r, nil
}

func (c *Client) SetExceptionBreakpoints(filters []string) error {
	req := &dap.SetExceptionBreakpointsRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "setExceptionBreakpoints"},
		Arguments: dap.SetExceptionBreakpointsArguments{Filters: filters},
	}
	_, err := c.Send(req)
	return err
}

func (c *Client) Disconnect(terminate bool) error {
	req := &dap.DisconnectRequest{
		Request:   dap.Request{ProtocolMessage: dap.ProtocolMessage{Type: "request"}, Command: "disconnect"},
		Arguments: &dap.DisconnectArguments{TerminateDebuggee: terminate},
	}
	_, err := c.Send(req)
	return err
}
