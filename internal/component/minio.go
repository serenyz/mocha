package component

import (
	"mmchat/internal/config"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func InitMinio(cfg *config.MinIOConfig) (*minio.Client, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv("MOCHA_MINIO_ACCESS_KEY"), os.Getenv("MOCHA_MINIO_SECRET_KEY"), ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})

	if err != nil {
		return nil, err
	}

	return client, err
}
