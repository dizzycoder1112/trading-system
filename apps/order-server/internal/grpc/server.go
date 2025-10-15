package grpc

import (
	"fmt"
	"net"

	"dizzycode.xyz/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	grpcServer *grpc.Server
	log        logger.Logger  // ✅ interface 傳值
}

// RegisterFunc 服務註冊函數
// 用於在創建 Server 時註冊 gRPC 服務
type RegisterFunc func(*grpc.Server)

// NewServer 創建 gRPC Server
// 參數：
//   - log: 日誌實例
//   - registers: 可變參數，用於註冊各種 gRPC 服務
//
// 範例：
//
//	grpcServer := NewServer(log,
//	    func(s *grpc.Server) {
//	        pb.RegisterOrderServiceServer(s, orderHandler)
//	    },
//	)
func NewServer(log logger.Logger, registers ...RegisterFunc) *Server {
	s := &Server{
		grpcServer: grpc.NewServer(),
		log:        log,
	}

	// 1. 先註冊所有服務
	for _, register := range registers {
		register(s.grpcServer)
	}

	// 2. 最後啟用 reflection（必須在服務註冊之後）
	reflection.Register(s.grpcServer)
	log.Debug("gRPC reflection enabled")

	return s
}

func (s *Server) Start(port string, ready chan<- struct{}) error {
	address := fmt.Sprintf(":%s", port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// 通知已準備好接受連接
	if ready != nil {
		close(ready)
	}

	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

func (s *Server) GracefulStop() {
	s.grpcServer.GracefulStop()
	s.log.Info("✅ gRPC server stopped")
}
