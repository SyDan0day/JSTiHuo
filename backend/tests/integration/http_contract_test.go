package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"
)

var contractPipe struct {
	sync.Mutex
	command *exec.Cmd
	input   io.WriteCloser
	output  *bufio.Reader
}

type contractResult struct {
	OK         bool     `json:"ok"`
	Operation  string   `json:"operation"`
	Status     int      `json:"status"`
	Errors     []string `json:"errors"`
	Operations []struct {
		ID       string `json:"id"`
		Success  int    `json:"success"`
		Rejected int    `json:"rejected"`
	} `json:"operations"`
}

// contractExchange 用途：串行通过内存管道校验真实 HTTP，不持久化业务正文。
// 参数：input 请求、响应或最终覆盖统计指令。
// 返回值：安全字段路径结果或工具错误，不包含凭据和收件资料。
func contractExchange(input any) (contractResult, error) {
	contractPipe.Lock()
	defer contractPipe.Unlock()
	var result contractResult
	if contractPipe.command == nil {
		command := exec.Command("node", "../../../tests/contract/validate-http.cjs")
		command.Stderr = io.Discard
		stdin, err := command.StdinPipe()
		if err != nil {
			return result, err
		}
		stdout, err := command.StdoutPipe()
		if err != nil {
			return result, err
		}
		if err = command.Start(); err != nil {
			return result, err
		}
		contractPipe.command, contractPipe.input, contractPipe.output = command, stdin, bufio.NewReader(stdout)
	}
	if err := json.NewEncoder(contractPipe.input).Encode(input); err != nil {
		return result, err
	}
	line, err := contractPipe.output.ReadBytes('\n')
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(line, &result)
	return result, err
}

// contractHTTP 用途：在显式契约模式下包装真实 Handler 并校验 JSON 交互。
// 参数：t 测试上下文；handler 原业务 Handler。
// 返回值：原 Handler 或契约观察器，业务结果保持原样。
func contractHTTP(t *testing.T, handler http.Handler) http.Handler {
	if os.Getenv("CONTRACT_VALIDATE") != "1" {
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal("契约请求读取失败")
		}
		defer clear(raw)
		r.Body = io.NopCloser(bytes.NewReader(raw))
		recorded := httptest.NewRecorder()
		handler.ServeHTTP(recorded, r)
		{
			var request, response any
			if json.Valid(raw) {
				if err = json.Unmarshal(raw, &request); err != nil {
					t.Fatal("契约请求解析失败")
				}
			}
			if json.Valid(recorded.Body.Bytes()) {
				response = json.RawMessage(recorded.Body.Bytes())
			}
			result, err := contractExchange(map[string]any{"method": r.Method, "path": r.URL.Path, "status": recorded.Code, "request": request, "response": response})
			if err != nil {
				t.Error("契约校验器不可用")
			} else if !result.OK {
				t.Errorf("契约不符 %s %d: %v", result.Operation, result.Status, result.Errors)
			}
		}
		for name, values := range recorded.Header() {
			w.Header()[name] = values
		}
		w.WriteHeader(recorded.Code)
		if recorded.Body.Len() > 0 {
			if _, err = w.Write(recorded.Body.Bytes()); err != nil {
				t.Error("契约响应复制失败")
			}
		}
	})
}

// TestMain 用途：执行集成测试并汇总契约实际正向与拒绝覆盖。
// 参数：m Go 测试运行器；CONTRACT_VALIDATE 显式启用内存校验。
// 返回值：无，通过进程退出码报告测试或契约错误。
func TestMain(m *testing.M) {
	code := m.Run()
	if os.Getenv("CONTRACT_VALIDATE") == "1" && contractPipe.command != nil {
		result, err := contractExchange(map[string]any{"finish": true})
		if err != nil || !result.OK {
			fmt.Fprintln(os.Stderr, "契约统计失败")
			code = 1
		} else {
			success := 0
			missing := []string{}
			for _, operation := range result.Operations {
				if operation.Success > 0 {
					success++
				} else {
					missing = append(missing, operation.ID)
				}
			}
			fmt.Printf("HTTP 契约：%d 个操作有真实成功响应；未观察到正向：%v\n", success, missing)
			if path := os.Getenv("CONTRACT_REPORT"); path != "" {
				raw, err := json.MarshalIndent(result.Operations, "", "  ")
				if err != nil || os.WriteFile(path, append(raw, '\n'), 0600) != nil {
					code = 1
				}
			}
			if os.Getenv("CONTRACT_REQUIRE_ALL") == "1" && len(missing) > 0 {
				code = 1
			}
		}
		contractPipe.input.Close()
		if err := contractPipe.command.Wait(); err != nil {
			code = 1
		}
	}
	os.Exit(code)
}
