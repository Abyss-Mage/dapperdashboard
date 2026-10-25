package main

import (
	"context"
	"dapper.local/platform/internal/email"
	"dapper.local/platform/internal/identity"
	"dapper.local/platform/internal/postgres"
	"dapper.local/platform/internal/square"
	"dapper.local/platform/internal/workspace"
	"encoding/base64"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

func main() {
	if os.Getenv("DAPPER_WORKSPACE") != "1" {
		log.Fatal("Set DAPPER_WORKSPACE=1 to start the Dapper application API")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := postgres.Open(ctx, os.Getenv("DAPPER_DATABASE_URL"))
	if err != nil {
		log.Fatal("Workspace database configuration unavailable; connection details are not logged")
	}
	defer store.Close()
	if err = store.ValidateSchema(ctx); err != nil {
		log.Fatal("Scoped API requires explicit schema migration and matching checksums; no automatic migration performed")
	}
	origin := "http://127.0.0.1:3006"
	if configured := os.Getenv("DAPPER_CLIENT_ORIGIN"); configured != "" {
		origin = configured
	}
	var invitations *postgres.InvitationService
	if configured := os.Getenv("DAPPER_INVITATION_ENCRYPTION_KEY"); configured != "" {
		key, err := base64.StdEncoding.DecodeString(configured)
		if err != nil || len(key) != 32 {
			log.Fatal("Invitation encryption requires an explicit valid 256-bit key; no key details are logged")
		}
		invitations, err = postgres.NewInvitationService(store, origin, key)
		clear(key)
		if err != nil {
			log.Fatal("Invitation encryption and HTTPS enrollment origin require valid configuration")
		}
	}
	var sqClient square.Client
	if sqToken := os.Getenv("DAPPER_SQUARE_ACCESS_TOKEN"); sqToken != "" {
		env := os.Getenv("DAPPER_SQUARE_ENVIRONMENT")
		var err error
		sqClient, err = square.NewClient(square.Config{
			AccessToken: sqToken,
			Environment: env,
		})
		if err != nil {
			log.Fatal("Square client initialization failed")
		}
	}
	credentials := map[string]postgres.SquareCredentialConfig{}
	if token := os.Getenv("DAPPER_SQUARE_SANDBOX_TOKEN"); token != "" {
		credentials["sandbox-default"] = postgres.SquareCredentialConfig{Environment: "sandbox", AccessToken: token}
	}
	if token := os.Getenv("DAPPER_SQUARE_PRODUCTION_TOKEN"); token != "" {
		credentials["production-default"] = postgres.SquareCredentialConfig{Environment: "production", AccessToken: token}
	}
	if resolver, err := postgres.NewSquareCredentialResolver(credentials); err != nil {
		log.Fatal("Square credential configuration is invalid")
	} else if resolver != nil {
		store.SetSquareCredentialResolver(resolver)
	}
	if configured := os.Getenv("DAPPER_MFA_KEY"); configured != "" {
		key, err := base64.StdEncoding.DecodeString(configured)
		if err != nil || store.SetMFAKey(key) != nil {
			log.Fatal("DAPPER_MFA_KEY must be a base64-encoded 256-bit key; no key details are logged")
		}
		clear(key)
	} else {
		log.Print("DAPPER_MFA_KEY is not set: authenticator-app MFA is unavailable and is not enforced")
	}
	uploadDir := os.Getenv("DAPPER_UPLOAD_DIR")
	if uploadDir == "" {
		log.Print("DAPPER_UPLOAD_DIR is not set: employee photo uploads are disabled")
	}
	store.SetUploadDirectory(uploadDir)
	go func() {
		// Weekly opening hours are materialized 12 weeks ahead; top them up at start and every 6 hours so availability never runs dry.
		for {
			if n, err := store.ExtendWeeklyHours(context.Background()); err != nil {
				log.Print("Weekly hours roll-forward could not complete; it will retry")
			} else if n > 0 {
				log.Printf("Weekly hours rolled forward for %d resources", n)
			}
			time.Sleep(6 * time.Hour)
		}
	}()
	go func() {
		// Approved retention: key tags expire after 30 days, customer-car photos never. Runs at start and every 6 hours.
		for {
			for {
				purged, err := store.PurgeExpiredUploads(context.Background(), 200)
				if err != nil {
					log.Print("Upload retention purge could not complete; it will retry")
					break
				}
				if purged > 0 {
					log.Printf("Upload retention purge removed %d expired key-tag posts", purged)
				}
				if purged < 200 {
					break
				}
			}
			time.Sleep(6 * time.Hour)
		}
	}()
	store.SetSOPDirectory(os.Getenv("DAPPER_SOP_DIR"))
	store.SetSquareApplicationIDs(map[string]string{
		"sandbox":    os.Getenv("DAPPER_SQUARE_SANDBOX_APPLICATION_ID"),
		"production": os.Getenv("DAPPER_SQUARE_PRODUCTION_APPLICATION_ID"),
	})
	// Franchises connect their own Square accounts with the application's secret (never logged, never sent to a browser).
	store.SetSquareOAuthApps(map[string]square.OAuthApp{
		"sandbox":    {Environment: "sandbox", ApplicationID: os.Getenv("DAPPER_SQUARE_SANDBOX_APPLICATION_ID"), Secret: os.Getenv("DAPPER_SQUARE_SANDBOX_APPLICATION_SECRET")},
		"production": {Environment: "production", ApplicationID: os.Getenv("DAPPER_SQUARE_PRODUCTION_APPLICATION_ID"), Secret: os.Getenv("DAPPER_SQUARE_PRODUCTION_APPLICATION_SECRET")},
	})
	go func() {
		// Square asks for OAuth access tokens to be renewed every 7 days or less; check every 6 hours.
		for {
			if _, err := store.RefreshSquareConnections(context.Background()); err != nil {
				log.Print("Square token renewal check failed")
			}
			time.Sleep(6 * time.Hour)
		}
	}()
	var emailSender postgres.EmailSender
	if smtpHost := os.Getenv("DAPPER_SMTP_HOST"); smtpHost != "" {
		smtpPort := 587
		if p, err := strconv.Atoi(os.Getenv("DAPPER_SMTP_PORT")); err == nil && p > 0 {
			smtpPort = p
		}
		var err error
		emailSender, err = email.NewSMTPSender(email.Config{
			Host:        smtpHost,
			Port:        smtpPort,
			Username:    os.Getenv("DAPPER_SMTP_USERNAME"),
			Password:    os.Getenv("DAPPER_SMTP_PASSWORD"),
			FromAddress: os.Getenv("DAPPER_SMTP_FROM_ADDRESS"),
			FromName:    os.Getenv("DAPPER_SMTP_FROM_NAME"),
		})
		if err != nil {
			log.Fatal("SMTP email sender initialization failed")
		}
	}
	sqWebhookURL := os.Getenv("DAPPER_SQUARE_WEBHOOK_URL")
	sqSignatureKey := os.Getenv("DAPPER_SQUARE_WEBHOOK_SIGNATURE_KEY")

	var handler http.Handler = workspace.NewHandlerWithFullServices(store, origin, invitations, sqClient, emailSender, sqWebhookURL, sqSignatureKey)
	if os.Getenv("DAPPER_PASSWORD_LOGIN") == "1" {
		lifetime, err := time.ParseDuration(os.Getenv("DAPPER_SESSION_LIFETIME"))
		if err != nil {
			log.Fatal("Explicit valid session lifetime required")
		}
		verifier, err := postgres.NewPasswordAuthenticator(store)
		if err != nil {
			log.Fatal("Password verification initialization failed")
		}
		login, err := identity.NewPasswordLogin(verifier, origin, lifetime)
		if err != nil {
			log.Fatal("Password sign-in requires an explicit HTTPS origin and session lifetime")
		}
		handler = workspace.NewHandlerWithFullServices(store, origin, invitations, sqClient, emailSender, sqWebhookURL, sqSignatureKey, login)
	} else if issuer := os.Getenv("DAPPER_OIDC_ISSUER"); issuer != "" {
		redirect, err := url.Parse(os.Getenv("DAPPER_OIDC_REDIRECT_URL"))
		if err != nil || redirect.Scheme != "https" || origin != redirect.Scheme+"://"+redirect.Host {
			log.Fatal("Configured login requires an exact HTTPS client origin matching its callback")
		}
		lifetime, err := time.ParseDuration(os.Getenv("DAPPER_SESSION_LIFETIME"))
		if err != nil {
			log.Fatal("Explicit valid session lifetime required")
		}
		login, err := identity.New(ctx, identity.Config{Issuer: issuer, ClientID: os.Getenv("DAPPER_OIDC_CLIENT_ID"), ClientSecret: os.Getenv("DAPPER_OIDC_CLIENT_SECRET"), RedirectURL: os.Getenv("DAPPER_OIDC_REDIRECT_URL"), SuccessURL: os.Getenv("DAPPER_LOGIN_SUCCESS_URL"), SessionLifetime: lifetime}, store)
		if err != nil {
			log.Fatal("Identity configuration unavailable; provider details and credentials are not logged")
		}
		handler = workspace.NewHandlerWithFullServices(store, origin, invitations, sqClient, emailSender, sqWebhookURL, sqSignatureKey, login)
	}
	server := &http.Server{Addr: "127.0.0.1:8121", Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	log.Print("Dapper application API listening on 127.0.0.1:8121; verified identity required; explicit migrations only")
	log.Fatal(server.ListenAndServe())
}
