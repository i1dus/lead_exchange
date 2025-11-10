package app

import (
	minio "lead_exchange/internal/lib/minio/core"

	"github.com/jackc/pgx/v5/pgxpool"

	grpcapp "lead_exchange/internal/app/grpc"
	"lead_exchange/internal/repository/user_repository"
	"lead_exchange/internal/services/user"

	"log/slog"
	"time"
)

type App struct {
	GRPCServer *grpcapp.App
}

func New(
	log *slog.Logger, grpcPort int, pool *pgxpool.Pool,
	tokenTTL time.Duration, secret string, minioClient minio.Client) *App {

	userRepository := user_repository.NewUserRepository(pool, log)

	userService := user.New(log, userRepository, tokenTTL, secret)

	grpcApp := grpcapp.New(log, userService, userService, minioClient, grpcPort, secret)

	return &App{
		GRPCServer: grpcApp,
	}
}
