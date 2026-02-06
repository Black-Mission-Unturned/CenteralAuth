package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BlackMission/centralauth/internal/auth"
	"github.com/BlackMission/centralauth/internal/client"
	"github.com/BlackMission/centralauth/internal/config"
	"github.com/BlackMission/centralauth/internal/domain"
	"github.com/BlackMission/centralauth/internal/exchange"
	"github.com/BlackMission/centralauth/internal/providers/discord"
	"github.com/BlackMission/centralauth/internal/providers/steam"
	"github.com/BlackMission/centralauth/internal/server"
	"github.com/BlackMission/centralauth/internal/state"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Build client registry
	clientApps := make([]domain.ClientApp, len(cfg.Clients))
	for i, c := range cfg.Clients {
		clientApps[i] = domain.ClientApp{
			ID:               c.ID,
			Name:             c.Name,
			APIKey:           c.APIKey,
			AllowedCallbacks: c.AllowedCallbacks,
			AllowedProviders: c.AllowedProviders,
		}
	}
	clients, err := client.NewRegistry(clientApps)
	if err != nil {
		log.Fatalf("failed to create client registry: %v", err)
	}

	// Build state service
	stateSvc := state.NewService([]byte(cfg.Secrets.StateSigningKey))

	// Build exchange codec
	encKey := []byte(cfg.Secrets.ExchangeEncryptionKey)
	if len(encKey) != 32 {
		log.Fatalf("exchange_encryption_key must be exactly 32 bytes, got %d", len(encKey))
	}
	codec, err := exchange.NewCodec(encKey)
	if err != nil {
		log.Fatalf("failed to create exchange codec: %v", err)
	}

	// Build provider registry
	providers := auth.NewRegistry()

	if dc, ok := cfg.Providers["discord"]; ok {
		callbackURL := cfg.Server.BaseURL + "/callback/discord"
		p := discord.New(discord.Config{
			ClientID:     dc.ClientID,
			ClientSecret: dc.ClientSecret,
			Scopes:       dc.Scopes,
			CallbackURL:  callbackURL,
		})
		if err := providers.Register(p); err != nil {
			log.Fatalf("failed to register discord provider: %v", err)
		}
		log.Println("Registered provider: discord")
	}

	if sc, ok := cfg.Providers["steam"]; ok {
		callbackURL := cfg.Server.BaseURL + "/callback/steam"
		p := steam.New(steam.Config{
			APIKey:      sc.APIKey,
			Realm:       sc.Realm,
			CallbackURL: callbackURL,
		})
		if err := providers.Register(p); err != nil {
			log.Fatalf("failed to register steam provider: %v", err)
		}
		log.Println("Registered provider: steam")
	}

	// Build and start server
	srv := server.New(server.Config{
		Host: cfg.Server.Host,
		Port: cfg.Server.Port,
	}, server.Deps{
		Clients:   clients,
		Providers: providers,
		State:     stateSvc,
		Exchange:  codec,
	})

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
