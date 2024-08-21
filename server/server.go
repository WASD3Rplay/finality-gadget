package server

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/babylonlabs-io/finality-gadget/config"
	"github.com/babylonlabs-io/finality-gadget/db"
	"github.com/babylonlabs-io/finality-gadget/finalitygadget"
	"github.com/lightningnetwork/lnd/signal"
	"go.uber.org/zap"

	"google.golang.org/grpc"
)

// Server is the main daemon construct for the finality gadget server. It handles
// spinning up the RPC sever, the database, and any other components that the
// the finality gadget server needs to run.
type Server struct {
	rpcServer *rpcServer
	cfg       *config.Config
	db        db.IDatabaseHandler

	logger *zap.Logger

	interceptor signal.Interceptor
	started     int32
}

// NewFinalityGadgetServer creates a new server with the given config.
func NewFinalityGadgetServer(cfg *config.Config, db db.IDatabaseHandler, fg finalitygadget.IFinalityGadget, sig signal.Interceptor, logger *zap.Logger) *Server {
	return &Server{
		cfg:         cfg,
		rpcServer:   newRPCServer(fg),
		db:          db,
		logger:      logger,
		interceptor: sig,
	}
}

// RunUntilShutdown runs the main finality gadget server loop until a signal is
// received to shut down the process.
func (s *Server) RunUntilShutdown() error {
	if atomic.AddInt32(&s.started, 1) != 1 {
		return nil
	}

	defer func() {
		s.logger.Info("Closing database...")
		s.db.Close()
		s.logger.Info("Database closed")
	}()

	// we create listeners from the GRPCListener defined in the config.
	lis, err := net.Listen("tcp", s.cfg.GRPCListener)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.cfg.GRPCListener, err)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer()
	defer grpcServer.Stop()

	if err := s.rpcServer.RegisterWithGrpcServer(grpcServer); err != nil {
		return fmt.Errorf("failed to register gRPC server: %w", err)
	}

	// All the necessary components have been registered, so we can
	// actually start listening for requests.
	if err := s.startGrpcListen(grpcServer, []net.Listener{lis}); err != nil {
		return fmt.Errorf("failed to start gRPC listener: %v", err)
	}

	s.logger.Info("Finality gadget is active")

	// Wait for shutdown signal from either a graceful server stop or from
	// the interrupt handler.
	<-s.interceptor.ShutdownChannel()

	return nil
}

// startGrpcListen starts the GRPC server on the passed listeners.
func (s *Server) startGrpcListen(grpcServer *grpc.Server, listeners []net.Listener) error {

	// Use a WaitGroup so we can be sure the instructions on how to input the
	// password is the last thing to be printed to the console.
	var wg sync.WaitGroup

	for _, lis := range listeners {
		wg.Add(1)
		go func(lis net.Listener) {
			s.logger.Info("RPC server listening", zap.String("address", lis.Addr().String()))

			// Close the ready chan to indicate we are listening.
			defer lis.Close()

			wg.Done()
			_ = grpcServer.Serve(lis)
		}(lis)
	}

	// Wait for gRPC servers to be up running.
	wg.Wait()

	return nil
}