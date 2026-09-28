package main

import (
	"context"
	"os"

	"github.com/gmlazutin/avito-go-template/internal/app"
	"github.com/gmlazutin/avito-go-template/internal/app/log"
)

func main() {
	logger := log.InitBootstrap(app.ServiceName)
	ctx := context.Background()

	application, err := app.New(ctx)
	if err != nil {
		logger.Error("initialize application", log.Error(err))
		os.Exit(1)
	}
	if err := application.Run(ctx); err != nil {
		logger.Error("run application", log.Error(err))
		os.Exit(1)
	}
}
