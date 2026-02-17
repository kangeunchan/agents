package v1

import (
	"os"
	"path/filepath"
	"strings"
)

type Manifest struct {
	APIVersion   string                    `yaml:"apiVersion" json:"apiVersion"`
	Metadata     Metadata                  `yaml:"metadata" json:"metadata"`
	Gateway      GatewayConfig             `yaml:"gateway" json:"gateway"`
	Providers    map[string]ProviderConfig `yaml:"providers,omitempty" json:"providers,omitempty"`
	Channels     map[string]ChannelConfig  `yaml:"channels,omitempty" json:"channels,omitempty"`
	Routing      RoutingConfig             `yaml:"routing,omitempty" json:"routing,omitempty"`
	Commands     map[string]any            `yaml:"commands,omitempty" json:"commands,omitempty"`
	Tools        map[string]any            `yaml:"tools,omitempty" json:"tools,omitempty"`
	ChatChannels map[string]map[string]any `yaml:"chatChannels,omitempty" json:"chatChannels,omitempty"`
	Agents       AgentsConfig              `yaml:"agents,omitempty" json:"agents,omitempty"`
	Plugins      PluginsConfig             `yaml:"plugins,omitempty" json:"plugins,omitempty"`
	Skills       SkillsConfig              `yaml:"skills,omitempty" json:"skills,omitempty"`
	Auth         AuthConfig                `yaml:"auth,omitempty" json:"auth,omitempty"`
	Logging      LoggingConfig             `yaml:"logging,omitempty" json:"logging,omitempty"`
	Runtime      RuntimeConfig             `yaml:"runtime,omitempty" json:"runtime,omitempty"`
}

type Metadata struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

type GatewayConfig struct {
	Mode              string           `yaml:"mode" json:"mode"`
	Bind              string           `yaml:"bind" json:"bind"`
	Port              int              `yaml:"port" json:"port"`
	TokenEnv          string           `yaml:"tokenEnv" json:"tokenEnv"`
	AllowUnconfigured bool             `yaml:"allowUnconfigured,omitempty" json:"allowUnconfigured,omitempty"`
	Transport         TransportConfig  `yaml:"transport,omitempty" json:"transport,omitempty"`
	TLS               TLSConfig        `yaml:"tls,omitempty" json:"tls,omitempty"`
	Health            HealthConfig     `yaml:"health,omitempty" json:"health,omitempty"`
	RateLimit         RateLimitConfig  `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
	Options           map[string]any   `yaml:"options,omitempty" json:"options,omitempty"`
	Headers           map[string]any   `yaml:"headers,omitempty" json:"headers,omitempty"`
	CORS              CORSConfig       `yaml:"cors,omitempty" json:"cors,omitempty"`
	Telemetry         TelemetryConfig  `yaml:"telemetry,omitempty" json:"telemetry,omitempty"`
	Metrics           MetricsConfig    `yaml:"metrics,omitempty" json:"metrics,omitempty"`
	Readiness         ReadinessConfig  `yaml:"readiness,omitempty" json:"readiness,omitempty"`
	Backoff           BackoffConfig    `yaml:"backoff,omitempty" json:"backoff,omitempty"`
	CircuitBreaker    CircuitBreaker   `yaml:"circuitBreaker,omitempty" json:"circuitBreaker,omitempty"`
	Concurrency       ConcurrencyLimit `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`
}

type TransportConfig struct {
	ReadTimeoutSeconds  int `yaml:"readTimeoutSeconds,omitempty" json:"readTimeoutSeconds,omitempty"`
	WriteTimeoutSeconds int `yaml:"writeTimeoutSeconds,omitempty" json:"writeTimeoutSeconds,omitempty"`
	IdleTimeoutSeconds  int `yaml:"idleTimeoutSeconds,omitempty" json:"idleTimeoutSeconds,omitempty"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	CertFile string `yaml:"certFile,omitempty" json:"certFile,omitempty"`
	KeyFile  string `yaml:"keyFile,omitempty" json:"keyFile,omitempty"`
}

type HealthConfig struct {
	Path             string `yaml:"path,omitempty" json:"path,omitempty"`
	IntervalSeconds  int    `yaml:"intervalSeconds,omitempty" json:"intervalSeconds,omitempty"`
	TimeoutSeconds   int    `yaml:"timeoutSeconds,omitempty" json:"timeoutSeconds,omitempty"`
	FailureThreshold int    `yaml:"failureThreshold,omitempty" json:"failureThreshold,omitempty"`
}

type RateLimitConfig struct {
	Enabled         bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	RequestsPerMin  int  `yaml:"requestsPerMin,omitempty" json:"requestsPerMin,omitempty"`
	Burst           int  `yaml:"burst,omitempty" json:"burst,omitempty"`
	PerTokenEnabled bool `yaml:"perTokenEnabled,omitempty" json:"perTokenEnabled,omitempty"`
}

type CORSConfig struct {
	Enabled        bool     `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	AllowedOrigins []string `yaml:"allowedOrigins,omitempty" json:"allowedOrigins,omitempty"`
	AllowedMethods []string `yaml:"allowedMethods,omitempty" json:"allowedMethods,omitempty"`
	AllowedHeaders []string `yaml:"allowedHeaders,omitempty" json:"allowedHeaders,omitempty"`
}

type TelemetryConfig struct {
	Enabled      bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	ServiceName  string `yaml:"serviceName,omitempty" json:"serviceName,omitempty"`
	Exporter     string `yaml:"exporter,omitempty" json:"exporter,omitempty"`
	Endpoint     string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	APIKeyEnv    string `yaml:"apiKeyEnv,omitempty" json:"apiKeyEnv,omitempty"`
	SampleRatio  int    `yaml:"sampleRatio,omitempty" json:"sampleRatio,omitempty"`
	TraceHeaders bool   `yaml:"traceHeaders,omitempty" json:"traceHeaders,omitempty"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Path    string `yaml:"path,omitempty" json:"path,omitempty"`
}

type ReadinessConfig struct {
	Enabled bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Path    string `yaml:"path,omitempty" json:"path,omitempty"`
}

type BackoffConfig struct {
	InitialMillis int `yaml:"initialMillis,omitempty" json:"initialMillis,omitempty"`
	MaxMillis     int `yaml:"maxMillis,omitempty" json:"maxMillis,omitempty"`
	JitterPct     int `yaml:"jitterPct,omitempty" json:"jitterPct,omitempty"`
}

type CircuitBreaker struct {
	Enabled             bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	FailureThresholdPct int  `yaml:"failureThresholdPct,omitempty" json:"failureThresholdPct,omitempty"`
	OpenSeconds         int  `yaml:"openSeconds,omitempty" json:"openSeconds,omitempty"`
	HalfOpenRequests    int  `yaml:"halfOpenRequests,omitempty" json:"halfOpenRequests,omitempty"`
}

type ConcurrencyLimit struct {
	Enabled      bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	MaxInflight  int  `yaml:"maxInflight,omitempty" json:"maxInflight,omitempty"`
	QueueSize    int  `yaml:"queueSize,omitempty" json:"queueSize,omitempty"`
	QueueTimeout int  `yaml:"queueTimeoutSeconds,omitempty" json:"queueTimeoutSeconds,omitempty"`
}

type ProviderConfig struct {
	Type      string         `yaml:"type" json:"type"`
	Model     string         `yaml:"model,omitempty" json:"model,omitempty"`
	Endpoint  string         `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	APIKeyEnv string         `yaml:"apiKeyEnv,omitempty" json:"apiKeyEnv,omitempty"`
	Region    string         `yaml:"region,omitempty" json:"region,omitempty"`
	Retry     RetryConfig    `yaml:"retry,omitempty" json:"retry,omitempty"`
	Timeout   TimeoutConfig  `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Options   map[string]any `yaml:"options,omitempty" json:"options,omitempty"`
}

type ChannelConfig struct {
	Provider       string         `yaml:"provider" json:"provider"`
	Model          string         `yaml:"model,omitempty" json:"model,omitempty"`
	Enabled        bool           `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Priority       int            `yaml:"priority,omitempty" json:"priority,omitempty"`
	MaxTokens      int            `yaml:"maxTokens,omitempty" json:"maxTokens,omitempty"`
	Temperature    float64        `yaml:"temperature,omitempty" json:"temperature,omitempty"`
	SystemPrompt   string         `yaml:"systemPrompt,omitempty" json:"systemPrompt,omitempty"`
	Headers        map[string]any `yaml:"headers,omitempty" json:"headers,omitempty"`
	Retry          RetryConfig    `yaml:"retry,omitempty" json:"retry,omitempty"`
	Timeout        TimeoutConfig  `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Streaming      bool           `yaml:"streaming,omitempty" json:"streaming,omitempty"`
	RequestOptions map[string]any `yaml:"requestOptions,omitempty" json:"requestOptions,omitempty"`
}

type RetryConfig struct {
	Attempts      int `yaml:"attempts,omitempty" json:"attempts,omitempty"`
	InitialMillis int `yaml:"initialMillis,omitempty" json:"initialMillis,omitempty"`
	MaxMillis     int `yaml:"maxMillis,omitempty" json:"maxMillis,omitempty"`
}

type TimeoutConfig struct {
	ConnectSeconds int `yaml:"connectSeconds,omitempty" json:"connectSeconds,omitempty"`
	ReadSeconds    int `yaml:"readSeconds,omitempty" json:"readSeconds,omitempty"`
	TotalSeconds   int `yaml:"totalSeconds,omitempty" json:"totalSeconds,omitempty"`
}

type RoutingConfig struct {
	DefaultChannel string      `yaml:"defaultChannel,omitempty" json:"defaultChannel,omitempty"`
	Rules          []RouteRule `yaml:"rules,omitempty" json:"rules,omitempty"`
}

type AgentsConfig struct {
	Defaults AgentDefaultsConfig `yaml:"defaults,omitempty" json:"defaults,omitempty"`
}

type AgentDefaultsConfig struct {
	ThinkingDefault string             `yaml:"thinkingDefault,omitempty" json:"thinkingDefault,omitempty"`
	MemorySearch    MemorySearchConfig `yaml:"memorySearch,omitempty" json:"memorySearch,omitempty"`
}

type MemorySearchConfig struct {
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

type RouteRule struct {
	Name       string   `yaml:"name,omitempty" json:"name,omitempty"`
	Match      Match    `yaml:"match,omitempty" json:"match,omitempty"`
	UseChannel string   `yaml:"useChannel" json:"useChannel"`
	Fallbacks  []string `yaml:"fallbacks,omitempty" json:"fallbacks,omitempty"`
}

type Match struct {
	ContainsAny []string `yaml:"containsAny,omitempty" json:"containsAny,omitempty"`
	ContainsAll []string `yaml:"containsAll,omitempty" json:"containsAll,omitempty"`
	TagsAny     []string `yaml:"tagsAny,omitempty" json:"tagsAny,omitempty"`
	TagsAll     []string `yaml:"tagsAll,omitempty" json:"tagsAll,omitempty"`
}

type PluginsConfig struct {
	Enabled bool                   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Allow   []string               `yaml:"allow,omitempty" json:"allow,omitempty"`
	Deny    []string               `yaml:"deny,omitempty" json:"deny,omitempty"`
	Entries map[string]PluginEntry `yaml:"entries,omitempty" json:"entries,omitempty"`
}

type PluginEntry struct {
	Enabled bool           `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Source  string         `yaml:"source,omitempty" json:"source,omitempty"`
	Version string         `yaml:"version,omitempty" json:"version,omitempty"`
	Config  map[string]any `yaml:"config,omitempty" json:"config,omitempty"`
}

type SkillsConfig struct {
	AllowBundled []string               `yaml:"allowBundled,omitempty" json:"allowBundled,omitempty"`
	DenyBundled  []string               `yaml:"denyBundled,omitempty" json:"denyBundled,omitempty"`
	Load         SkillsLoadConfig       `yaml:"load,omitempty" json:"load,omitempty"`
	Entries      map[string]SkillConfig `yaml:"entries,omitempty" json:"entries,omitempty"`
}

type SkillsLoadConfig struct {
	ExtraDirs []string `yaml:"extraDirs,omitempty" json:"extraDirs,omitempty"`
}

type SkillConfig struct {
	Enabled bool              `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Command string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

type AuthConfig struct {
	TokenEnv     string   `yaml:"tokenEnv,omitempty" json:"tokenEnv,omitempty"`
	AllowedCIDRs []string `yaml:"allowedCIDRs,omitempty" json:"allowedCIDRs,omitempty"`
}

type LoggingConfig struct {
	Level      string         `yaml:"level,omitempty" json:"level,omitempty"`
	Format     string         `yaml:"format,omitempty" json:"format,omitempty"`
	File       string         `yaml:"file,omitempty" json:"file,omitempty"`
	Rotation   Rotation       `yaml:"rotation,omitempty" json:"rotation,omitempty"`
	Structured bool           `yaml:"structured,omitempty" json:"structured,omitempty"`
	Fields     map[string]any `yaml:"fields,omitempty" json:"fields,omitempty"`
}

type Rotation struct {
	MaxSizeMB  int  `yaml:"maxSizeMB,omitempty" json:"maxSizeMB,omitempty"`
	MaxBackups int  `yaml:"maxBackups,omitempty" json:"maxBackups,omitempty"`
	MaxAgeDays int  `yaml:"maxAgeDays,omitempty" json:"maxAgeDays,omitempty"`
	Compress   bool `yaml:"compress,omitempty" json:"compress,omitempty"`
}

type RuntimeConfig struct {
	ContainerName      string `yaml:"containerName,omitempty" json:"containerName,omitempty"`
	Image              string `yaml:"image,omitempty" json:"image,omitempty"`
	ComposeFile        string `yaml:"composeFile,omitempty" json:"composeFile,omitempty"`
	ConfigPath         string `yaml:"configPath,omitempty" json:"configPath,omitempty"`
	StateDir           string `yaml:"stateDir,omitempty" json:"stateDir,omitempty"`
	StateVolume        string `yaml:"stateVolume,omitempty" json:"stateVolume,omitempty"`
	WorkspaceDir       string `yaml:"workspaceDir,omitempty" json:"workspaceDir,omitempty"`
	ExecApprovalPreset string `yaml:"execApprovalPreset,omitempty" json:"execApprovalPreset,omitempty"`
}

func (m *Manifest) ApplyDefaults() {
	if m.APIVersion == "" {
		m.APIVersion = "openclawctl/v1"
	}
	if m.Gateway.Mode == "" {
		m.Gateway.Mode = "local"
	}
	if m.Gateway.Bind == "" {
		m.Gateway.Bind = "127.0.0.1"
	}
	if m.Gateway.Port == 0 {
		m.Gateway.Port = 18789
	}
	if m.Gateway.TokenEnv == "" {
		m.Gateway.TokenEnv = "OPENCLAW_GATEWAY_TOKEN"
	}
	if m.Gateway.Health.Path == "" {
		m.Gateway.Health.Path = "/health"
	}
	if m.Gateway.Health.IntervalSeconds == 0 {
		m.Gateway.Health.IntervalSeconds = 30
	}
	if m.Gateway.Health.TimeoutSeconds == 0 {
		m.Gateway.Health.TimeoutSeconds = 5
	}
	if m.Gateway.Health.FailureThreshold == 0 {
		m.Gateway.Health.FailureThreshold = 3
	}
	if m.Logging.Level == "" {
		m.Logging.Level = "info"
	}
	if m.Logging.Format == "" {
		m.Logging.Format = "json"
	}
	if m.Runtime.ContainerName == "" {
		m.Runtime.ContainerName = "openclaw-gateway"
	}
	if m.Runtime.Image == "" {
		m.Runtime.Image = "openclaw-gateway:local"
	}
	if m.Runtime.ComposeFile == "" {
		m.Runtime.ComposeFile = "openclaw/docker-compose.dev.yaml"
	}
	if m.Runtime.ConfigPath == "" {
		m.Runtime.ConfigPath = defaultRuntimeConfigPath()
	}
	if m.Runtime.StateDir == "" {
		m.Runtime.StateDir = "/var/lib/openclaw"
	}
	if m.Runtime.StateVolume == "" {
		m.Runtime.StateVolume = "openclaw_state"
	}
}

func defaultRuntimeConfigPath() string {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".openclawctl", "openclaw.json")
	}
	return ".openclawctl/openclaw.json"
}
