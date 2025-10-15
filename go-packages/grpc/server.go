package grpc

import (
	"fmt"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Server 通用的 gRPC Server 封裝
// 用於管理 gRPC Server 的生命週期
type Server struct {
	grpcServer *grpc.Server
	log        Logger  // 使用自己的 Logger interface
}

// RegisterFunc 服務註冊函數
// 用於在創建 Server 時註冊 gRPC 服務
type RegisterFunc func(*grpc.Server)

// NewServer 創建 gRPC Server
//
// 參數：
//   - log: 日誌實例（可選，如果為 nil 會使用默認的 console logger）
//   - registers: 可變參數，用於註冊各種 gRPC 服務
//
// 範例：
//
//	grpcServer := grpc.NewServer(log,
//	    func(s *grpc.Server) {
//	        pb.RegisterOrderServiceServer(s, orderHandler)
//	    },
//	    func(s *grpc.Server) {
//	        pb.RegisterUserServiceServer(s, userHandler)
//	    },
//	)
//
// 特性：
//   - 自動按順序註冊所有服務
//   - 自動啟用 gRPC reflection（方便 grpcurl/grpcui 測試）
//   - 支援優雅關閉
//   - 如果不提供 logger，會使用內建的 console logger
func NewServer(log Logger, registers ...RegisterFunc) *Server {
	// 使用默認 logger 作為 fallback
	if log == nil {
		log = defaultLog
	}

	s := &Server{
		grpcServer: grpc.NewServer(),
		log:        log,
	}

	// 1. 先註冊所有服務
	for _, register := range registers {
		register(s.grpcServer)
	}

	// 2. 最後啟用 reflection（必須在服務註冊之後）
	// reflection 讓 grpcurl 和 grpcui 可以動態發現服務
	reflection.Register(s.grpcServer)
	log.Debug("gRPC reflection enabled")

	return s
}

// Start 啟動 gRPC Server
//
// 參數：
//   - port: 監聽端口（例如："50051" 或 "8080"）
//   - ready: 可選的通道，Server 準備好後會關閉此通道（用於測試或協調啟動）
//
// 這個方法會阻塞直到 Server 停止或發生錯誤
func (s *Server) Start(port string, ready chan<- struct{}) error {
	address := fmt.Sprintf(":%s", port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	s.log.Info("gRPC server listening", map[string]any{
		"address": address,
	})

	// 通知已準備好接受連接（可選）
	if ready != nil {
		close(ready)
	}

	// 開始接受連接（阻塞）
	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// GracefulStop 優雅關閉 gRPC Server
//
// 會等待所有正在處理的請求完成後再關閉
// 適合在收到 SIGTERM 或 SIGINT 信號時調用
func (s *Server) GracefulStop() {
	s.log.Info("Gracefully stopping gRPC server...")
	s.grpcServer.GracefulStop()
	s.log.Info("gRPC server stopped")
}

// Stop 立即停止 gRPC Server
//
// 不等待正在處理的請求，立即關閉所有連接
// 一般不推薦使用，除非緊急情況
func (s *Server) Stop() {
	s.log.Warn("Force stopping gRPC server...")
	s.grpcServer.Stop()
	s.log.Info("gRPC server stopped")
}
