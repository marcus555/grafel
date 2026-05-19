package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/cajasmota/archigraph/internal/daemon/proto"
)

// Config configures Run. Fields are required unless documented otherwise.
type Config struct {
	Layout  Layout      // on-disk paths (see DefaultLayout)
	Index   IndexFunc   // injected from cmd/archigraph
	Rebuild RebuildFunc // injected from cmd/archigraph
	Logger  *log.Logger // optional; defaults to stderr
}

// Run starts the daemon. It blocks until either:
//   - the Service receives Stop,
//   - the process receives SIGTERM/SIGINT, or
//   - the listener errors fatally.
//
// On exit it removes the socket file and pid file. The function is the
// daemon's entire public surface — cmd/archigraph just imports daemon
// and calls Run.
func Run(ctx context.Context, cfg Config) error {
	logger := cfg.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "archigraph-daemon: ", log.LstdFlags|log.Lmicroseconds)
	}

	if err := EnsureLayout(cfg.Layout); err != nil {
		return fmt.Errorf("ensure layout: %w", err)
	}

	releasePID, err := AcquirePIDFile(cfg.Layout.PIDPath)
	if err != nil {
		return err
	}
	defer releasePID()

	// Remove any stale socket from a previous crash. We checked the
	// pid file above so we know no live daemon is using it.
	_ = os.Remove(cfg.Layout.SocketPath)

	listener, err := net.Listen("unix", cfg.Layout.SocketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.Layout.SocketPath, err)
	}
	// 0600 makes the socket per-user; the daemon is single-user only.
	if err := os.Chmod(cfg.Layout.SocketPath, 0o600); err != nil {
		_ = listener.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(cfg.Layout.SocketPath)
	}()

	stopReq := make(chan struct{})
	svc := newService(cfg.Index, cfg.Rebuild, cfg.Layout.SocketPath, stopReq)

	server := rpc.NewServer()
	if err := server.RegisterName(proto.ServiceName, svc); err != nil {
		return fmt.Errorf("register %s: %w", proto.ServiceName, err)
	}

	// Signals — we want SIGTERM (systemd, launchd's stop) and SIGINT
	// (Ctrl-C when running in the foreground for development).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	logger.Printf("ready socket=%s pid=%d", cfg.Layout.SocketPath, os.Getpid())

	// Track accepted connections so we can wait for them to drain on
	// shutdown. The waitgroup is decremented when each conn loop returns.
	var connWG sync.WaitGroup
	acceptDone := make(chan struct{})
	go acceptLoop(listener, server, &connWG, logger, acceptDone)

	// Wait for any shutdown trigger.
	select {
	case <-stopReq:
		logger.Printf("stop requested via RPC")
	case sig := <-sigCh:
		logger.Printf("signal %s received", sig)
	case <-ctx.Done():
		logger.Printf("context cancelled: %v", ctx.Err())
	case <-acceptDone:
		// acceptLoop only returns when the listener closes, which we
		// don't do until shutdown — but if the listener dies on its
		// own we should treat that as fatal and exit.
		logger.Printf("listener closed unexpectedly")
		return errors.New("listener closed")
	}

	// Stop accepting new connections, then wait for in-flight ones.
	_ = listener.Close()
	<-acceptDone
	connWG.Wait()
	logger.Printf("graceful shutdown complete")
	return nil
}

// acceptLoop pulls connections off the listener and hands each to
// jsonrpc.ServeConn under the registered server. The waitgroup tracks
// each conn so Run can join them on shutdown.
func acceptLoop(l net.Listener, srv *rpc.Server, wg *sync.WaitGroup, logger *log.Logger, done chan<- struct{}) {
	defer close(done)
	for {
		conn, err := l.Accept()
		if err != nil {
			// Listener closed during shutdown — that's the happy path.
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logger.Printf("accept: %v", err)
			return
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			defer c.Close()
			srv.ServeCodec(jsonrpc.NewServerCodec(&loggingConn{Conn: c, log: logger}))
		}(conn)
	}
}

// loggingConn wraps a net.Conn so EOF / read errors get a single log
// line. Without this, jsonrpc swallows the close silently and we have
// no way to confirm clients are actually disconnecting on demand.
type loggingConn struct {
	net.Conn
	log *log.Logger
}

func (c *loggingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if err != nil && err != io.EOF {
		// EOF is the normal client disconnect; anything else is worth
		// noting. We don't return the wrapper here, so jsonrpc still
		// sees the original error.
		c.log.Printf("conn read: %v", err)
	}
	return n, err
}
