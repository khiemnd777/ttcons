package core

import (
	"errors"
	"strings"
	"sync"
	"time"

	"consflow/backend/internal/shared/mailer"
	sharedstorage "consflow/backend/internal/shared/storage"
)

type Service struct {
	repo                        *Repository
	jwtSecret                   string
	projectExplorer             *projectExplorerRuntime
	streamHub                   *ActivityStreamHub
	streamCapacity              *ActivityStreamCapacity
	streamVisibilityTimeout     time.Duration
	streamMaxConnectionLifetime time.Duration
	maxUploadBytes              int64
	storageMu                   sync.Mutex
	storageRegistry             *sharedstorage.Registry
	storageWriteProvider        string
	storageConfigErr            error
	accessTokenTTL              time.Duration
	refreshTokenTTL             time.Duration
	refreshCookieSecure         bool
	refreshCookieSameSite       string
	recoveryEnabled             bool
	recoveryTokenTTL            time.Duration
	recoveryVerifyTTL           time.Duration
	frontendOrigin              string
	mailSender                  mailer.Sender
	recoveryDeliverySlots       chan struct{}
}

func NewService(repo *Repository, jwtSecret string) *Service {
	return &Service{
		repo:                        repo,
		jwtSecret:                   jwtSecret,
		streamHub:                   NewActivityStreamHub(),
		streamCapacity:              NewActivityStreamCapacity(500, 20, 2),
		streamVisibilityTimeout:     2 * time.Second,
		streamMaxConnectionLifetime: 5 * time.Minute,
		maxUploadBytes:              defaultMaxUploadBytes,
		storageWriteProvider:        sharedstorage.LocalProvider,
		storageConfigErr:            errors.New("storage registry is not configured"),
		accessTokenTTL:              defaultAccessTokenTTL,
		refreshTokenTTL:             defaultRefreshTokenTTL,
		refreshCookieSameSite:       defaultRefreshCookieSameSite,
		recoveryTokenTTL:            15 * time.Minute,
		recoveryVerifyTTL:           24 * time.Hour,
		recoveryDeliverySlots:       make(chan struct{}, 16),
	}
}

const (
	defaultMaxUploadBytes          = int64(10 * 1024 * 1024)
	defaultAccessTokenTTL          = 15 * time.Minute
	defaultRefreshTokenTTL         = 7 * 24 * time.Hour
	defaultRefreshCookieSameSite   = "Lax"
	activityMessageEditWindow      = 15 * time.Minute
	maxSealUploadBytes             = int64(10 * 1024 * 1024)
	maxSealImageDimension          = 4096
	maxSealImagePixels             = 16 * 1024 * 1024
	maxProfileAvatarUploadBytes    = int64(2 * 1024 * 1024)
	maxProfileAvatarImageDimension = 2048
	maxProfileAvatarImagePixels    = 4 * 1024 * 1024
)

func (s *Service) ConfigureUploads(uploadDir string, maxUploadBytes int64) {
	uploadDir = strings.TrimSpace(uploadDir)
	if uploadDir == "" {
		s.storageMu.Lock()
		s.storageRegistry = nil
		s.storageConfigErr = errors.New("storage path is empty")
		s.storageMu.Unlock()
		return
	}
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMaxUploadBytes
	}
	localStore, err := sharedstorage.NewLocal(uploadDir)
	if err != nil {
		s.storageMu.Lock()
		s.maxUploadBytes = maxUploadBytes
		s.storageRegistry = nil
		s.storageConfigErr = err
		s.storageMu.Unlock()
		return
	}
	registry, err := sharedstorage.NewRegistry(localStore)
	if err == nil {
		err = s.ConfigureStorage(registry, sharedstorage.LocalProvider, maxUploadBytes)
	}
	s.storageMu.Lock()
	if err != nil {
		s.storageRegistry = nil
	}
	s.storageConfigErr = err
	s.storageMu.Unlock()
}

func (s *Service) ConfigureStorage(registry *sharedstorage.Registry, writeProvider string, maxUploadBytes int64) error {
	if registry == nil {
		return errors.New("storage registry is nil")
	}
	writeProvider = strings.ToLower(strings.TrimSpace(writeProvider))
	if writeProvider == "" {
		writeProvider = sharedstorage.LocalProvider
	}
	if _, err := registry.Store(writeProvider); err != nil {
		return err
	}
	if maxUploadBytes <= 0 {
		maxUploadBytes = defaultMaxUploadBytes
	}
	s.storageMu.Lock()
	s.storageRegistry = registry
	s.storageWriteProvider = writeProvider
	s.maxUploadBytes = maxUploadBytes
	s.storageConfigErr = nil
	s.storageMu.Unlock()
	return nil
}

func (s *Service) storageBackend() (*sharedstorage.Registry, error) {
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	if s.storageRegistry != nil {
		return s.storageRegistry, nil
	}
	if s.storageConfigErr != nil {
		return nil, s.storageConfigErr
	}
	return nil, errors.New("storage registry is not configured")
}

func (s *Service) storageWriter() (*sharedstorage.Registry, string, error) {
	registry, err := s.storageBackend()
	if err != nil {
		return nil, "", err
	}
	s.storageMu.Lock()
	provider := s.storageWriteProvider
	s.storageMu.Unlock()
	if strings.TrimSpace(provider) == "" {
		provider = sharedstorage.LocalProvider
	}
	return registry, provider, nil
}

func (s *Service) ConfigureAuth(accessTokenMinutes int64, refreshTokenHours int64, refreshCookieSecure bool, refreshCookieSameSite string) {
	if accessTokenMinutes <= 0 {
		accessTokenMinutes = int64(defaultAccessTokenTTL / time.Minute)
	}
	if refreshTokenHours <= 0 {
		refreshTokenHours = int64(defaultRefreshTokenTTL / time.Hour)
	}
	refreshCookieSameSite = strings.TrimSpace(refreshCookieSameSite)
	if refreshCookieSameSite == "" {
		refreshCookieSameSite = defaultRefreshCookieSameSite
	}
	s.accessTokenTTL = time.Duration(accessTokenMinutes) * time.Minute
	s.refreshTokenTTL = time.Duration(refreshTokenHours) * time.Hour
	s.refreshCookieSecure = refreshCookieSecure
	s.refreshCookieSameSite = refreshCookieSameSite
}

func (s *Service) ConfigureAccountRecovery(enabled bool, sender mailer.Sender, frontendOrigin string, tokenTTL, verifyTTL time.Duration) {
	if tokenTTL <= 0 {
		tokenTTL = 15 * time.Minute
	}
	if verifyTTL <= 0 {
		verifyTTL = 24 * time.Hour
	}
	s.recoveryEnabled = enabled
	s.mailSender = sender
	s.frontendOrigin = strings.TrimRight(strings.TrimSpace(frontendOrigin), "/")
	s.recoveryTokenTTL = tokenTTL
	s.recoveryVerifyTTL = verifyTTL
}
