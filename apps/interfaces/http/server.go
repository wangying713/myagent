package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"saklay/ai/apps/config"
)

const httpShutdownTimeout = 10 * time.Second

func runHTTP(ctx context.Context, handler http.Handler, cfg config.HTTPSettings) error {
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("监听 HTTP 地址 %q 失败: %w", cfg.Address, err)
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      cfg.RunTimeout + 10*time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	fmt.Fprintln(os.Stdout, "Go 后台接口已启动：http://"+listener.Addr().String())
	return serveHTTP(ctx, server, listener, httpShutdownTimeout)
}

// serveHTTP 在信号取消和监听失败时执行相同的有界清理。
// Serve 结束只说明监听循环退出，业务请求由 requestTracker 单独跟踪。
func serveHTTP(ctx context.Context, server *http.Server, listener net.Listener, timeout time.Duration) error {
	requestCtx, cancelRequests := context.WithCancel(ctx)
	defer cancelRequests()
	requests := &requestTracker{handler: server.Handler, done: make(chan struct{})}
	server.Handler = requests
	server.BaseContext = func(net.Listener) context.Context { return requestCtx }
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()

	var result error
	serveStopped := false
	select {
	case err := <-serveErrors:
		result = serveError(err)
		serveStopped = true
	case <-ctx.Done():
	}
	cancelRequests()
	drained := requests.stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		result = errors.Join(result, fmt.Errorf("停止 HTTP 服务失败: %w", err))
		if closeErr := server.Close(); closeErr != nil {
			result = errors.Join(result, fmt.Errorf("强制关闭 HTTP 连接失败: %w", closeErr))
		}
	}
	if !serveStopped {
		select {
		case err := <-serveErrors:
			result = errors.Join(result, serveError(err))
		default:
			select {
			case err := <-serveErrors:
				result = errors.Join(result, serveError(err))
			case <-shutdownCtx.Done():
				result = errors.Join(result, fmt.Errorf("等待 HTTP 监听结束失败: %w", shutdownCtx.Err()))
			}
		}
	}
	// 即使连接已关闭，处理方法也可能尚未返回；不能只等待 Serve。
	select {
	case <-drained:
		return result
	default:
	}
	select {
	case <-drained:
		return result
	case <-shutdownCtx.Done():
		pending := requests.pending()
		if pending == 0 {
			// 请求可能与截止信号同时结束；停止闸门保证此后不会再增加。
			return result
		}
		return errors.Join(result, fmt.Errorf("HTTP 退出未完成，仍有 %d 个请求运行，后续观测可能缺失: %w", pending, shutdownCtx.Err()))
	}
}

func serveError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("HTTP 服务异常退出: %w", err)
}

type requestTracker struct {
	handler  http.Handler
	mu       sync.Mutex
	active   int
	stopping bool
	done     chan struct{}
}

func (t *requestTracker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	if t.stopping {
		t.mu.Unlock()
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		// 退出期间连接可能已关闭，写入失败不应延长退出或触发业务重试。
		_, _ = io.WriteString(w, `{"data":null,"error":{"code":"service_unavailable","message":"服务正在退出，请稍后重试"}}`)
		return
	}
	t.active++
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.active--
		if t.stopping && t.active == 0 {
			close(t.done)
		}
	}()
	t.handler.ServeHTTP(w, r)
}

func (t *requestTracker) stop() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.stopping {
		t.stopping = true
		if t.active == 0 {
			close(t.done)
		}
	}
	return t.done
}

func (t *requestTracker) pending() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.active
}
