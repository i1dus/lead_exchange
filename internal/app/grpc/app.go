package grpcapp

import (
	"context"
	"crypto/tls"
	"fmt"
	"lead_exchange/internal/grpc/authgrpc"
	"lead_exchange/internal/grpc/filegrpc"
	"lead_exchange/internal/grpc/leadgrpc"
	"lead_exchange/internal/grpc/usergrpc"
	minio "lead_exchange/internal/lib/minio/core"
	"lead_exchange/internal/middleware"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	pb "lead_exchange/pkg"

	"github.com/go-chi/chi/v5"
	"github.com/rs/cors"
	httpSwagger "github.com/swaggo/http-swagger"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type App struct {
	log        *slog.Logger
	gRPCServer *grpc.Server
	port       int
}

// New создаёт gRPC + HTTP (Gateway) сервер с Auth, User и File сервисами.
func New(
	log *slog.Logger,
	authSvc authgrpc.AuthService,
	userSvc usergrpc.UserService,
	minioClient minio.Client,
	leadSvc leadgrpc.LeadService,
	port int,
	secret string,
) *App {
	loggingOpts := []logging.Option{
		logging.WithLogOnEvents(logging.PayloadReceived, logging.PayloadSent),
	}
	recoveryOpts := []recovery.Option{
		recovery.WithRecoveryHandler(func(p interface{}) (err error) {
			log.Error("Recovered from panic", slog.Any("panic", p))
			return status.Errorf(codes.Internal, "internal error")
		}),
	}

	gRPCServer := grpc.NewServer(grpc.ChainUnaryInterceptor(
		recovery.UnaryServerInterceptor(recoveryOpts...),
		logging.UnaryServerInterceptor(InterceptorLogger(log), loggingOpts...),
		middleware.JWTUnaryInterceptor(secret),
	))

	// ✅ Регистрируем все три gRPC сервера
	authgrpc.RegisterAuthServerGRPC(gRPCServer, authSvc)
	usergrpc.RegisterUserServerGRPC(gRPCServer, userSvc)
	filegrpc.RegisterFileServerGRPC(gRPCServer, minioClient)
	leadgrpc.RegisterLeadServerGRPC(gRPCServer, leadSvc)

	return &App{
		log:        log,
		gRPCServer: gRPCServer,
		port:       port,
	}
}

func InterceptorLogger(l *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	const op = "grpcapp.Run"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 2)

	// === gRPC Server ===
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	// === HTTP Gateway ===
	gwMux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	for _, register := range []func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error{
		pb.RegisterAuthServiceHandlerFromEndpoint,
		pb.RegisterUserServiceHandlerFromEndpoint,
		pb.RegisterFileServiceHandlerFromEndpoint,
		pb.RegisterLeadServiceHandlerFromEndpoint,
	} {
		if err := register(ctx, gwMux, fmt.Sprintf("localhost:%d", a.port), opts); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
	}

	// === Swagger UI ===
	httpMux := http.NewServeMux()
	httpMux.Handle("/", gwMux)

	// Маршруты для трёх swagger-файлов
	swaggerMux := chi.NewMux()

	swaggerFiles := map[string]string{
		"/swagger/auth/doc.json": "pkg/auth.swagger.json",
		"/swagger/user/doc.json": "pkg/user.swagger.json",
		"/swagger/file/doc.json": "pkg/file.swagger.json",
		"/swagger/lead/doc.json": "pkg/lead.swagger.json",
	}

	// Отдаём swagger.json для каждого сервиса
	for route, path := range swaggerFiles {
		swPath := path // копия для замыкания
		swaggerMux.HandleFunc(route, func(w http.ResponseWriter, _ *http.Request) {
			b, err := os.ReadFile(swPath)
			if err != nil {
				http.Error(w, fmt.Sprintf("swagger file not found: %s", swPath), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(b)
		})
	}

	// Общий Swagger UI с выбором любого json
	swaggerMux.HandleFunc("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/auth/doc.json"), // по умолчанию откроется Auth
		httpSwagger.URL("/swagger/user/doc.json"),
		httpSwagger.URL("/swagger/file/doc.json"),
		httpSwagger.URL("/swagger/lead/doc.json"),
	))

	httpMux.Handle("/swagger/", swaggerMux)

	httpServer := &http.Server{
		Addr:         ":8081",
		Handler:      cors.AllowAll().Handler(httpMux),
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}

	// === Запускаем gRPC ===
	go func() {
		a.log.Info("gRPC server started", slog.String("addr", grpcListener.Addr().String()))
		if err := a.gRPCServer.Serve(grpcListener); err != nil && err != grpc.ErrServerStopped {
			errCh <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// === Запускаем HTTP Gateway ===
	go func() {
		a.log.Info("HTTP server started", slog.String("addr", httpServer.Addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		a.log.Info("shutdown signal received")
	case err := <-errCh:
		cancel()
		return fmt.Errorf("%s: %w", op, err)
	}

	a.log.Info("shutting down servers")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	var shutdownErr error
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		shutdownErr = fmt.Errorf("HTTP shutdown error: %w", err)
	}
	a.gRPCServer.GracefulStop()

	return shutdownErr
}

func (a *App) Stop() {
	a.log.Info("stopping gRPC server", slog.Int("port", a.port))
	a.gRPCServer.GracefulStop()
}
