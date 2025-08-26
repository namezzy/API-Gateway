package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"api-gateway/internal/config"
	"api-gateway/internal/gateway"
	"api-gateway/internal/logger"
)

var (
	configFile = flag.String("config", "configs/config.yaml", "配置文件路径")
	version    = flag.Bool("version", false, "显示版本信息")
)

const (
	appVersion = "1.0.0"
	appName    = "API Gateway"
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("%s v%s\n", appName, appVersion)
		os.Exit(0)
	}

	// 加载配置
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	if err := logger.Init(cfg.Logging); err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	logger.Infof("启动 %s v%s", appName, appVersion)
	logger.Infof("配置文件: %s", *configFile)

	// 创建网关实例
	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		logger.Fatalf("创建网关实例失败: %v", err)
	}

	// 启动指标服务器（如果启用）
	var metricsServer *http.Server
	if cfg.Metrics.Enabled {
		metricsServer = startMetricsServer(cfg.Metrics.Port, cfg.Metrics.Path)
	}

	// 创建上下文用于优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动网关（在goroutine中）
	serverErr := make(chan error, 1)
	go func() {
		if err := gw.Start(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("网关启动失败: %w", err)
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待退出信号或启动错误
	select {
	case err := <-serverErr:
		logger.Errorf("服务器错误: %v", err)
	case sig := <-sigChan:
		logger.Infof("接收到信号: %s，开始优雅关闭", sig)
	}

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	logger.Info("正在关闭服务...")

	// 关闭网关
	if err := gw.Stop(shutdownCtx); err != nil {
		logger.Errorf("关闭网关失败: %v", err)
	}

	// 关闭指标服务器
	if metricsServer != nil {
		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			logger.Errorf("关闭指标服务器失败: %v", err)
		}
	}

	logger.Info("服务已停止")
}

// startMetricsServer 启动指标服务器
func startMetricsServer(port int, path string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle(path, promhttp.Handler())
	
	// 添加健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"metrics"}`))
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		logger.Infof("指标服务器启动在端口 %d，路径 %s", port, path)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("指标服务器启动失败: %v", err)
		}
	}()

	return server
}
