# gRPC Server Package

通用的 gRPC Server 封裝，提供生命週期管理和服務註冊功能。

## 特性

- ✅ **完全獨立**：無外部 logger 依賴，內建 fallback logger
- ✅ **靈活註冊**：透過 `RegisterFunc` 動態註冊服務
- ✅ **自動 Reflection**：自動啟用 gRPC reflection（方便測試）
- ✅ **優雅關閉**：支援 graceful shutdown
- ✅ **類型安全**：使用 Go 泛型和函數式編程

## 快速開始

### 基本使用

```go
package main

import (
    "dizzycoder.xyz/grpc"
    pb "your-project/proto"
)

func main() {
    // 創建 handler
    orderHandler := NewOrderHandler()

    // 創建 gRPC server（使用默認 console logger）
    server := grpc.NewServer(nil,
        func(s *grpc.Server) {
            pb.RegisterOrderServiceServer(s, orderHandler)
        },
    )

    // 啟動 server
    server.Start("50051", nil)
}
```

### 使用自定義 Logger

```go
import (
    "dizzycoder.xyz/grpc"
    "dizzycoder.xyz/logger"
)

func main() {
    // 使用你的 logger 實現
    log := logger.NewZap(...)

    // 創建 gRPC server
    server := grpc.NewServer(log,
        func(s *grpc.Server) {
            pb.RegisterOrderServiceServer(s, orderHandler)
        },
    )

    server.Start("50051", nil)
}
```

### Logger Adapter

如果你的 logger 介面不同，可以創建 adapter：

```go
// adapter.go
package main

import (
    "dizzycoder.xyz/grpc"
    "go.uber.org/zap"
)

// ZapLoggerAdapter 將 zap.Logger 適配到 grpc.Logger
type ZapLoggerAdapter struct {
    zap *zap.Logger
}

func NewZapAdapter(z *zap.Logger) *ZapLoggerAdapter {
    return &ZapLoggerAdapter{zap: z}
}

func (a *ZapLoggerAdapter) Info(msg string, context ...any) {
    a.zap.Sugar().Infow(msg, context...)
}

func (a *ZapLoggerAdapter) Error(msg string, context ...any) {
    a.zap.Sugar().Errorw(msg, context...)
}

func (a *ZapLoggerAdapter) Debug(msg string, context ...any) {
    a.zap.Sugar().Debugw(msg, context...)
}

func (a *ZapLoggerAdapter) Warn(msg string, context ...any) {
    a.zap.Sugar().Warnw(msg, context...)
}

// 使用
func main() {
    zapLogger, _ := zap.NewProduction()
    adapter := NewZapAdapter(zapLogger)

    server := grpc.NewServer(adapter,
        func(s *grpc.Server) {
            pb.RegisterOrderServiceServer(s, orderHandler)
        },
    )

    server.Start("50051", nil)
}
```

## API

### `NewServer(log Logger, registers ...RegisterFunc) *Server`

創建 gRPC Server。

**參數**：
- `log`: Logger 實例（可選，nil 則使用默認 console logger）
- `registers`: 可變參數，服務註冊函數

**返回**：
- `*Server`: Server 實例

### `(*Server) Start(port string, ready chan<- struct{}) error`

啟動 gRPC Server（阻塞）。

**參數**：
- `port`: 監聽端口（例如："50051"）
- `ready`: 可選通道，Server 準備好後會關閉

**返回**：
- `error`: 啟動失敗時的錯誤

### `(*Server) GracefulStop()`

優雅關閉 Server（等待所有請求完成）。

### `(*Server) Stop()`

立即停止 Server（不等待請求完成）。

## 完整範例

```go
package main

import (
    "os"
    "os/signal"
    "syscall"

    "dizzycoder.xyz/grpc"
    "dizzycoder.xyz/logger"
    pb "your-project/proto"
)

func main() {
    // 1. 創建 logger
    log := logger.Must(config.Load())

    // 2. 創建 handlers
    orderHandler := NewOrderHandler()
    userHandler := NewUserHandler()

    // 3. 創建 gRPC server
    server := grpc.NewServer(log,
        func(s *grpc.Server) {
            pb.RegisterOrderServiceServer(s, orderHandler)
            log.Info("OrderService registered")
        },
        func(s *grpc.Server) {
            pb.RegisterUserServiceServer(s, userHandler)
            log.Info("UserService registered")
        },
    )

    // 4. 在 goroutine 中啟動
    go func() {
        if err := server.Start("50051", nil); err != nil {
            log.Error("Server failed", map[string]any{"error": err})
            os.Exit(1)
        }
    }()

    log.Info("Server started on :50051")

    // 5. 等待關閉信號
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    // 6. 優雅關閉
    log.Info("Shutting down...")
    server.GracefulStop()
}
```

## 設計哲學

1. **完全獨立**：像 `websocket` 包一樣，不依賴外部 logger
2. **靈活可擴展**：透過函數式編程支援動態註冊
3. **開箱即用**：提供默認實現，也支援注入
4. **符合 Go 慣例**：簡單、直接、易於理解

## 與其他包的對比

| 特性 | go-packages/grpc | google.golang.org/grpc |
|------|-----------------|------------------------|
| 服務註冊 | 函數式，靈活 | 手動調用 Register 方法 |
| Reflection | 自動啟用 | 需手動啟用 |
| 生命週期管理 | 內建 Start/Stop | 需自己管理 |
| Logger | 可選注入 | 需自己整合 |
| 優雅關閉 | 內建支援 | 需自己實現 |

## License

MIT
