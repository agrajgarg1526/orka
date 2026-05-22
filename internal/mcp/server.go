package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/agrajgarg/orka/internal/state"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func respond(id interface{}, result interface{}) response {
	return response{JSONRPC: "2.0", ID: id, Result: result}
}

func respondErr(id interface{}, code int, msg string) response {
	return response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

// Serve runs the MCP server on stdin/stdout until EOF.
func Serve(st *state.State, statePath string) error {
	enc := json.NewEncoder(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = enc.Encode(respondErr(nil, -32700, "parse error"))
			continue
		}

		var resp response
		switch req.Method {
		case "list_tasks":
			resp = respond(req.ID, map[string]interface{}{"tasks": st.Tasks})

		case "get_task":
			var p struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(req.Params, &p)
			var found *state.Task
			for i := range st.Tasks {
				if st.Tasks[i].ID == p.ID {
					found = &st.Tasks[i]
					break
				}
			}
			if found == nil {
				resp = respondErr(req.ID, 404, fmt.Sprintf("task %q not found", p.ID))
			} else {
				resp = respond(req.ID, found)
			}

		case "complete_phase":
			var p struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(req.Params, &p)
			advanced := false
			for i := range st.Tasks {
				if st.Tasks[i].ID == p.ID {
					next := st.Tasks[i].NextPhase()
					if next != "" {
						st.UpdateTaskPhase(p.ID, next)
						_ = st.Save(statePath)
					}
					resp = respond(req.ID, map[string]string{"status": "ok"})
					advanced = true
					break
				}
			}
			if !advanced {
				resp = respondErr(req.ID, 404, fmt.Sprintf("task %q not found", p.ID))
			}

		case "report_error":
			var p struct {
				ID      string `json:"id"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(req.Params, &p)
			st.SetTaskError(p.ID, p.Message)
			_ = st.Save(statePath)
			resp = respond(req.ID, map[string]string{"status": "ok"})

		case "update_notes":
			var p struct {
				ID   string `json:"id"`
				Text string `json:"text"`
			}
			_ = json.Unmarshal(req.Params, &p)
			updated := false
			for i := range st.Tasks {
				if st.Tasks[i].ID == p.ID {
					st.Tasks[i].Notes += "\n" + time.Now().Format("15:04") + " " + p.Text
					_ = st.Save(statePath)
					resp = respond(req.ID, map[string]string{"status": "ok"})
					updated = true
					break
				}
			}
			if !updated {
				resp = respondErr(req.ID, 404, fmt.Sprintf("task %q not found", p.ID))
			}

		default:
			resp = respondErr(req.ID, -32601, fmt.Sprintf("method %q not found", req.Method))
		}

		_ = enc.Encode(resp)
	}
	return scanner.Err()
}
