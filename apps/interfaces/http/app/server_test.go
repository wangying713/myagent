package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"saklay/ai/apps/config"
	"saklay/ai/domain/agent"
)

// httpRun 保存测试 HTTP 服务的生命周期状态。
type httpRun struct {
	server *http.Server       // 当前测试启动的 HTTP 服务器。
	cancel context.CancelFunc // 触发服务停止的取消函数。
	done   chan struct{}      // 服务生命周期函数返回后关闭的通知通道。
	err    error              // 服务退出结果，在 done 关闭后读取。
}

func listenForTest(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	})
	return listener
}

func startHTTPForTest(t *testing.T, listener net.Listener, handler http.Handler, timeout time.Duration) *httpRun {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	run := &httpRun{
		server: &http.Server{Handler: handler},
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go func() {
		run.err = serveHTTP(ctx, run.server, listener, timeout)
		close(run.done)
	}()
	t.Cleanup(func() {
		cancel()
		waitHTTPForTest(t, run)
	})
	return run
}

func waitHTTPForTest(t *testing.T, run *httpRun) error {
	t.Helper()
	select {
	case <-run.done:
		return run.err
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP 服务未在测试期限内退出")
		return nil
	}
}

// httpCall 保存异步测试请求的完成通知与错误。
type httpCall struct {
	done chan struct{} // 请求执行结束后关闭的通知通道。
	err  error         // 请求执行错误，在 done 关闭后读取。
}

func requestForTest(t *testing.T, listener net.Listener, path, body string) *httpCall {
	t.Helper()
	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{
		Transport: transport,
		Timeout:   2 * time.Second,
	}
	call := &httpCall{done: make(chan struct{})}
	go func() {
		defer close(call.done)
		request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+listener.Addr().String()+path, strings.NewReader(body))
		if err != nil {
			call.err = err
			return
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			call.err = err
			return
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		call.err = errors.Join(readErr, response.Body.Close())
	}()
	t.Cleanup(func() {
		select {
		case <-call.done:
		case <-time.After(3 * time.Second):
			t.Error("测试 HTTP 客户端未退出")
		}
	})
	return call
}

func waitSignal(t *testing.T, done <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("等待%s超时", operation)
	}
}

// failedAcceptListener 将监听失败替换为指定错误，验证服务退出行为。
type failedAcceptListener struct {
	net.Listener       // 实际接收连接的底层监听器。
	failure      error // 底层接收失败时替换返回的测试错误。
}

func (l *failedAcceptListener) Accept() (net.Conn, error) {
	connection, err := l.Listener.Accept()
	if err != nil {
		return nil, l.failure
	}
	return connection, nil
}

func TestServeHTTPCancelsAndJoinsRequests(t *testing.T) {
	for _, acceptFailure := range []bool{
		false,
		true,
	} {
		t.Run(map[bool]string{
			false: "退出信号",
			true:  "监听异常",
		}[acceptFailure], func(t *testing.T) {
			listener := listenForTest(t)
			failure := errors.New("测试监听失败")
			var serving net.Listener = listener
			if acceptFailure {
				serving = &failedAcceptListener{
					Listener: listener,
					failure:  failure,
				}
			}
			started, finished := make(chan struct{}), make(chan struct{})
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(finished)
				close(started)
				<-r.Context().Done()
				w.WriteHeader(http.StatusOK)
			})
			run := startHTTPForTest(t, serving, handler, time.Second)
			call := requestForTest(t, listener, "/", "")
			waitSignal(t, started, "请求开始")
			if acceptFailure {
				if err := listener.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				run.cancel()
			}
			err := waitHTTPForTest(t, run)
			if acceptFailure && !errors.Is(err, failure) || !acceptFailure && err != nil {
				t.Errorf("实际退出错误 %v，预期保留监听异常或正常退出", err)
			}
			select {
			case <-finished:
			default:
				t.Error("HTTP 服务先于请求处理结束而返回")
			}
			waitSignal(t, call.done, "客户端结束")
			if call.err != nil {
				t.Errorf("协作退出的请求未完整返回：%v", call.err)
			}
		})
	}
}

func TestServeHTTPReportsUnfinishedRequestsWithoutHanging(t *testing.T) {
	for _, acceptFailure := range []bool{
		false,
		true,
	} {
		t.Run(map[bool]string{
			false: "退出信号",
			true:  "监听异常",
		}[acceptFailure], func(t *testing.T) {
			listener := listenForTest(t)
			failure := errors.New("测试监听失败")
			var serving net.Listener = listener
			if acceptFailure {
				serving = &failedAcceptListener{
					Listener: listener,
					failure:  failure,
				}
			}
			started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
			run := startHTTPForTest(t, serving, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(finished)
				close(started)
				<-release // 故意不响应取消，验证退出期限而非业务实现。
			}), 20*time.Millisecond)
			t.Cleanup(func() {
				close(release)
				waitSignal(t, finished, "释放未结束的请求")
			})
			call := requestForTest(t, listener, "/", "")
			waitSignal(t, started, "请求开始")
			if acceptFailure {
				if err := listener.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				run.cancel()
			}
			err := waitHTTPForTest(t, run)
			if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "仍有 1 个请求运行") {
				t.Fatalf("实际退出错误 %v，预期报告超时与未完成请求", err)
			}
			if acceptFailure && !errors.Is(err, failure) {
				t.Error("退出超时覆盖了原始监听异常")
			}
			select {
			case <-finished:
				t.Error("测试处理器本应仍在等待释放")
			default:
			}
			waitSignal(t, call.done, "强制关闭客户端连接")
			if call.err == nil {
				t.Error("退出超时没有关闭正在等待的连接")
			}
		})
	}
}

func TestRequestTrackerRejectsLateRequestsAndWaitsForActiveOnes(t *testing.T) {
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	tracker := &requestTracker{
		done: make(chan struct{}),
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-release
		}),
	}
	go func() {
		tracker.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		close(finished)
	}()
	t.Cleanup(func() {
		close(release)
		waitSignal(t, finished, "在途请求结束")
		waitSignal(t, tracker.stop(), "请求跟踪结束")
	})
	waitSignal(t, started, "请求开始")
	drained := tracker.stop()
	select {
	case <-drained:
		t.Fatal("尚有请求运行时提前判定退出完成")
	default:
	}
	out := httptest.NewRecorder()
	tracker.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/", nil))
	if out.Code != 503 || !strings.Contains(out.Body.String(), "service_unavailable") || out.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("退出期间的新请求实际响应 %d %s，预期固定 JSON 503", out.Code, out.Body.String())
	}
}

func TestServeHTTPWaitsAfterConnectionsAreClosed(t *testing.T) {
	listener := listenForTest(t)
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	run := startHTTPForTest(t, listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		close(started)
		<-release
	}), 20*time.Millisecond)
	t.Cleanup(func() {
		close(release)
		waitSignal(t, finished, "连接关闭后的请求结束")
	})
	call := requestForTest(t, listener, "/", "")
	waitSignal(t, started, "请求开始")
	if err := run.server.Close(); err != nil {
		t.Fatal(err)
	}
	err := waitHTTPForTest(t, run)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "仍有 1 个请求运行") {
		t.Fatalf("实际退出错误 %v，预期连接关闭后仍等待处理器并报告未完成", err)
	}
	waitSignal(t, call.done, "客户端连接关闭")
}

func TestRunHTTPPreservesListenError(t *testing.T) {
	listener := listenForTest(t)
	err := runHTTP(t.Context(), http.NotFoundHandler(), config.HTTPSettings{
		Address:    listener.Addr().String(),
		RunTimeout: time.Second,
	})
	var listenError *net.OpError
	if !errors.As(err, &listenError) || listenError.Op != "listen" {
		t.Fatalf("实际监听错误 %v，预期保留 net.OpError", err)
	}
}

func TestServeHTTPFinishesSpansBeforeReturning(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	exporter := &memoryExporter{}
	handler, telemetry := setup(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}, routerTestConfig{
		Limits:  agent.DefaultLimits(),
		Timeout: 5 * time.Second,
	}, exporter)
	t.Cleanup(func() {
		close(release)
	})
	listener := listenForTest(t)
	run := startHTTPForTest(t, listener, handler, time.Second)
	call := requestForTest(t, listener, "/api/agent/chat", `{}`)
	waitSignal(t, started, "模型开始")
	run.cancel()
	if err := waitHTTPForTest(t, run); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, call.done, "聊天请求结束")
	flushCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := telemetry.Flush(flushCtx); err != nil {
		t.Fatal(err)
	}
	exporter.mu.Lock()
	defer exporter.mu.Unlock()
	if len(exporter.spans) != 3 {
		t.Errorf("HTTP 返回后实际收集 %d 个 span，预期 HTTP、运行与模型的 3 个 span 均已结束", len(exporter.spans))
	}
}
