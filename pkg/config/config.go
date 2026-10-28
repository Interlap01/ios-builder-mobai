// Package config exposes mobai.json handling to code outside this module.
//
// The implementation lives in internal/config, which Go forbids other modules
// from importing. This package re-exports it so ios-builder can be consumed as
// a library. Types are aliases, not wrappers, so values pass between the two
// packages without conversion.
package config

import "github.com/MobAI-App/ios-builder/internal/config"

// ConfigFileName is the default configuration file name.
const ConfigFileName = config.ConfigFileName

// ErrConfigNotFound indicates mobai.json was not found.
var ErrConfigNotFound = config.ErrConfigNotFound

type (
	Config            = config.Config
	GitHubConfig      = config.GitHubConfig
	CIConfig          = config.CIConfig
	IOSConfig         = config.IOSConfig
	FlutterConfig     = config.FlutterConfig
	WatchConfig       = config.WatchConfig
	ReactNativeConfig = config.ReactNativeConfig
	KMPConfig         = config.KMPConfig
	MobAIConfig       = config.MobAIConfig
	SigningConfig     = config.SigningConfig
	Profile           = config.Profile
	BuildSettings     = config.BuildSettings
	SigningSecrets    = config.SigningSecrets
	ValidationError   = config.ValidationError
	Manager           = config.Manager
)

// Distributions a build profile can name. Empty means an unsigned build.
const (
	DistributionDevelopment = config.DistributionDevelopment
	DistributionAdHoc       = config.DistributionAdHoc
	DistributionStore       = config.DistributionStore
	DistributionEnterprise  = config.DistributionEnterprise
)

// NewManager creates a configuration manager for ./mobai.json.
func NewManager() *Manager {
	return config.NewManager()
}

// MergeInto writes cfg into existing mobai.json content and returns the new
// file. cfg is all of ios-builder's part, so start from the file's content:
// its keys are set from cfg and removed when cfg leaves them empty (no empty
// sections), every key ios-builder does not know is kept, and an empty Name
// keeps the file's "name".
func MergeInto(existing []byte, cfg *Config) ([]byte, error) { return config.MergeInto(existing, cfg) }

// ParseDistribution canonicalizes a distribution name (internal is ad-hoc).
func ParseDistribution(s string) (string, error) { return config.ParseDistribution(s) }

// SigningSet is the secret suffix of a distribution: STORE, AD_HOC, DEVELOPMENT.
func SigningSet(distribution string) (string, error) { return config.SigningSet(distribution) }

// SigningSecretNames are the four secrets of a signing set.
func SigningSecretNames(set string) SigningSecrets { return config.SigningSecretNames(set) }
