package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/HP-jh/TSGMemOptimizer/internal/config"
	"github.com/HP-jh/TSGMemOptimizer/internal/engine"
	"github.com/HP-jh/TSGMemOptimizer/internal/server"
)

const version = "1.3.0"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("TSGMemOptimizer v%s starting on %s/%s...", version, runtime.GOOS, runtime.GOARCH)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load: %v", err)
	}
	cfg.Version = version
	_ = config.Save(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mon := engine.NewMonitor()
	mon.Start(ctx)
	defer mon.Stop()

	pm := engine.NewProcessManager()
	pm.Start(ctx)
	defer pm.Stop()

	cleaner := engine.NewCleaner()

	srv := server.New(mon, cleaner, pm, cfg)
	srv.RunBackground(ctx)

	port := os.Getenv("TSGMEMOPORT")
	if port == "" {
		port = "29471"
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port

	httpSrv := &http.Server{
		Handler:      srv.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		url := fmt.Sprintf("http://127.0.0.1:%d", actualPort)
		log.Printf("UI ready at %s", url)
		// try open browser
		_ = openBrowser(url)
	}()

	go func() {
		if err := httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	cancel()
	log.Println("goodbye")
}

func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{"open", url}
	case "windows":
		cmd, args = "cmd", []string{"cmd", "/c", "start", url}
	default:
		cmd, args = "xdg-open", []string{"xdg-open", url}
	}
	_, err := os.StartProcess(cmd, args, &os.ProcAttr{})
	return err
}
