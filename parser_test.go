package envParser

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestEnvironToMap(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    map[string]string
		wantErr bool
	}{
		{"valid", []string{"KEY=value", "FOO=bar"}, map[string]string{"KEY": "value", "FOO": "bar"}, false},
		{"empty value", []string{"KEY="}, map[string]string{"KEY": ""}, false},
		{"value with equals", []string{"KEY=val=ue"}, map[string]string{"KEY": "val=ue"}, false},
		{"invalid format", []string{"NOEQUALS"}, nil, true},
		{"empty input", []string{}, map[string]string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EnvironToMap(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnvironToMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != len(tt.want) {
				t.Errorf("EnvironToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseEnvFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"simple", "KEY=value", []string{"KEY=value"}},
		{"multiline", "KEY=value\nFOO=bar", []string{"KEY=value", "FOO=bar"}},
		{"with comments", "# comment\nKEY=value", []string{"KEY=value"}},
		{"empty lines", "KEY=value\n\nFOO=bar", []string{"KEY=value", "FOO=bar"}},
		{"windows crlf", "KEY=value\r\nFOO=bar\r\n", []string{"KEY=value", "FOO=bar"}},
		{"whitespace", "  KEY=value  \n  # comment  ", []string{"KEY=value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEnvFile(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("parseEnvFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	type Config struct {
		Host    string        `env:"HOST"`
		Port    int           `env:"PORT"`
		Debug   bool          `env:"DEBUG"`
		Timeout time.Duration `env:"TIMEOUT"`
		Rate    float64       `env:"RATE"`
	}

	envs := map[string]string{
		"HOST":    "localhost",
		"PORT":    "8080",
		"DEBUG":   "true",
		"TIMEOUT": "5s",
		"RATE":    "1.5",
	}

	var cfg Config
	if err := Unmarshal(envs, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if cfg.Host != "localhost" || cfg.Port != 8080 || !cfg.Debug || cfg.Timeout != 5*time.Second || cfg.Rate != 1.5 {
		t.Errorf("Unmarshal() = %+v", cfg)
	}
}

func TestUnmarshalDefaults(t *testing.T) {
	type Config struct {
		Host string `env:"HOST,default=localhost"`
		Port int    `env:"PORT,default=3000"`
	}

	var cfg Config
	if err := Unmarshal(map[string]string{}, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if cfg.Host != "localhost" || cfg.Port != 3000 {
		t.Errorf("defaults not applied: %+v", cfg)
	}
}

func TestUnmarshalRequired(t *testing.T) {
	type Config struct {
		Host string `env:"HOST,required"`
	}

	var cfg Config
	err := Unmarshal(map[string]string{}, &cfg)
	if err == nil {
		t.Error("expected error for missing required field")
	}
}

func TestUnmarshalSlice(t *testing.T) {
	type Config struct {
		Hosts []string `env:"HOSTS,separator=|"`
		Ports []int    `env:"PORTS,separator=|"`
	}

	envs := map[string]string{"HOSTS": "a|b|c", "PORTS": "80|443"}
	var cfg Config
	if err := Unmarshal(envs, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(cfg.Hosts) != 3 || len(cfg.Ports) != 2 {
		t.Errorf("slices not parsed: %+v", cfg)
	}
}

func TestUnmarshalNestedStruct(t *testing.T) {
	type DB struct {
		Host string `env:"DB_HOST"`
	}
	type Config struct {
		DB DB
	}

	envs := map[string]string{"DB_HOST": "dbserver"}
	var cfg Config
	if err := Unmarshal(envs, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if cfg.DB.Host != "dbserver" {
		t.Errorf("nested struct not parsed: %+v", cfg)
	}
}

func TestUnmarshalPointer(t *testing.T) {
	type Config struct {
		Value *string `env:"VALUE"`
	}

	envs := map[string]string{"VALUE": "test"}
	var cfg Config
	if err := Unmarshal(envs, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if cfg.Value == nil || *cfg.Value != "test" {
		t.Errorf("pointer not parsed: %+v", cfg)
	}
}

func TestUnmarshalUint(t *testing.T) {
	type Config struct {
		Count uint64 `env:"COUNT"`
	}

	envs := map[string]string{"COUNT": "42"}
	var cfg Config
	if err := Unmarshal(envs, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if cfg.Count != 42 {
		t.Errorf("uint not parsed: %+v", cfg)
	}
}

func TestUnmarshalFromFile(t *testing.T) {
	content := "# Config file\nTEST_KEY=test_value\n\nTEST_NUM=123\n"
	f, err := os.CreateTemp("", "env")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	type Config struct {
		Key string `env:"TEST_KEY"`
		Num int    `env:"TEST_NUM"`
	}

	var cfg Config
	if err := UnmarshalFromFile(f.Name(), &cfg); err != nil {
		t.Fatalf("UnmarshalFromFile() error = %v", err)
	}

	if cfg.Key != "test_value" || cfg.Num != 123 {
		t.Errorf("file not parsed: %+v", cfg)
	}
}

func TestParseTag(t *testing.T) {
	tests := []struct {
		tag      string
		key      string
		required bool
		defVal   string
	}{
		{"KEY", "KEY", false, ""},
		{"KEY,required", "KEY", true, ""},
		{"KEY,default=val", "KEY", false, "val"},
		{"KEY,required,default=val", "KEY", true, "val"},
		{"KEY,default", "KEY", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			tf := parseTag(tt.tag)
			if tf.Key != tt.key || tf.Required != tt.required || tf.Default != tt.defVal {
				t.Errorf("parseTag(%q) = %+v", tt.tag, tf)
			}
		})
	}
}

func TestUnmarshalErrors(t *testing.T) {
	var s string
	if err := Unmarshal(map[string]string{}, s); err != ErrInvalidValue {
		t.Error("expected ErrInvalidValue for non-pointer")
	}
	if err := Unmarshal(map[string]string{}, nil); err != ErrInvalidValue {
		t.Error("expected ErrInvalidValue for nil")
	}
	if err := Unmarshal(map[string]string{}, &s); err != ErrInvalidValue {
		t.Error("expected ErrInvalidValue for non-struct pointer")
	}
}

func TestUnmarshalMap(t *testing.T) {
	type Config struct {
		Labels map[string]string `env:"LABELS"`
		Ports  map[string]int    `env:"PORTS"`
	}

	envs := map[string]string{
		"LABELS": "env:prod;region:us",
		"PORTS":  "http:80;https:443",
	}
	var cfg Config
	if err := Unmarshal(envs, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if cfg.Labels["env"] != "prod" || cfg.Labels["region"] != "us" {
		t.Errorf("string map not parsed: %+v", cfg.Labels)
	}
	if cfg.Ports["http"] != 80 || cfg.Ports["https"] != 443 {
		t.Errorf("int map not parsed: %+v", cfg.Ports)
	}
}

func TestUnmarshalMapEmpty(t *testing.T) {
	type Config struct {
		Labels map[string]string `env:"LABELS"`
	}

	envs := map[string]string{"LABELS": ""}
	var cfg Config
	if err := Unmarshal(envs, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if cfg.Labels == nil {
		t.Error("empty map should be initialized, not nil")
	}
}

func TestUnmarshalEscapedCommaSeparator(t *testing.T) {
	type Config struct {
		Items []string `env:"ITEMS,separator=\\,"`
	}

	envs := map[string]string{"ITEMS": "a,b,c"}
	var cfg Config
	if err := Unmarshal(envs, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if len(cfg.Items) != 3 || cfg.Items[0] != "a" || cfg.Items[1] != "b" || cfg.Items[2] != "c" {
		t.Errorf("escaped comma separator not working: %+v", cfg.Items)
	}
}

func TestParseTagEscapedComma(t *testing.T) {
	tf := parseTag(`KEY,separator=\,`)
	if tf.Key != "KEY" || tf.Separator != "," {
		t.Errorf("parseTag with escaped comma = %+v", tf)
	}

	tf2 := parseTag(`KEY,default=a\,b`)
	if tf2.Default != "a,b" {
		t.Errorf("parseTag default with escaped comma = %+v", tf2)
	}
}

func TestUnmarshalComprehensive(t *testing.T) {
	type Database struct {
		Host string `env:"DB_HOST,required"`
		Port int    `env:"DB_PORT,default=5432"`
	}

	type Config struct {
		AppName     string            `env:"APP_NAME,required"`
		Debug       bool              `env:"DEBUG,default=false"`
		Port        int               `env:"PORT,default=8080"`
		Timeout     time.Duration     `env:"TIMEOUT,default=30s"`
		Rate        float64           `env:"RATE,default=1.5"`
		MaxRetries  uint              `env:"MAX_RETRIES,default=3"`
		Hosts       []string          `env:"HOSTS,separator=|"`
		Ports       []int             `env:"PORTS,separator=|"`
		Labels      map[string]string `env:"LABELS"`
		Counts      map[string]int    `env:"COUNTS"`
		OptionalPtr *string           `env:"OPTIONAL_PTR"`
		Database    Database
		NoTag       string
	}

	envs := map[string]string{
		"APP_NAME":     "myapp",
		"DEBUG":        "true",
		"PORT":         "3000",
		"TIMEOUT":      "1m",
		"RATE":         "2.5",
		"MAX_RETRIES":  "5",
		"HOSTS":        "host1|host2|host3",
		"PORTS":        "80|443|8080",
		"LABELS":       "env:prod;version:v1",
		"COUNTS":       "errors:10;warnings:5",
		"OPTIONAL_PTR": "optional",
		"DB_HOST":      "localhost",
		"DB_PORT":      "3306",
	}

	var cfg Config
	if err := Unmarshal(envs, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if cfg.AppName != "myapp" {
		t.Errorf("AppName = %v, want myapp", cfg.AppName)
	}
	if !cfg.Debug {
		t.Error("Debug should be true")
	}
	if cfg.Port != 3000 {
		t.Errorf("Port = %v, want 3000", cfg.Port)
	}
	if cfg.Timeout != time.Minute {
		t.Errorf("Timeout = %v, want 1m", cfg.Timeout)
	}
	if cfg.Rate != 2.5 {
		t.Errorf("Rate = %v, want 2.5", cfg.Rate)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %v, want 5", cfg.MaxRetries)
	}
	if len(cfg.Hosts) != 3 || cfg.Hosts[1] != "host2" {
		t.Errorf("Hosts = %v", cfg.Hosts)
	}
	if len(cfg.Ports) != 3 || cfg.Ports[2] != 8080 {
		t.Errorf("Ports = %v", cfg.Ports)
	}
	if cfg.Labels["env"] != "prod" || cfg.Labels["version"] != "v1" {
		t.Errorf("Labels = %v", cfg.Labels)
	}
	if cfg.Counts["errors"] != 10 || cfg.Counts["warnings"] != 5 {
		t.Errorf("Counts = %v", cfg.Counts)
	}
	if cfg.OptionalPtr == nil || *cfg.OptionalPtr != "optional" {
		t.Errorf("OptionalPtr = %v", cfg.OptionalPtr)
	}
	if cfg.Database.Host != "localhost" || cfg.Database.Port != 3306 {
		t.Errorf("Database = %+v", cfg.Database)
	}
}

type mockValidator struct {
	called bool
	err    error
}

func (m *mockValidator) Struct(v interface{}) error {
	m.called = true
	return m.err
}

func TestValidatorIntegration(t *testing.T) {
	defer SetValidator(nil)

	type Config struct {
		Name string `env:"NAME"`
	}

	t.Run("validator called when set", func(t *testing.T) {
		mv := &mockValidator{}
		SetValidator(mv)

		envs := map[string]string{"NAME": "test"}
		var cfg Config
		_ = Unmarshal(envs, &cfg)

		if !mv.called {
			t.Error("validator.Struct was not called")
		}
	})

	t.Run("validator error returned", func(t *testing.T) {
		mv := &mockValidator{err: errors.New("validation failed")}
		SetValidator(mv)

		envs := map[string]string{"NAME": "test"}
		var cfg Config
		err := Unmarshal(envs, &cfg)

		if err == nil || err.Error() != "validation failed" {
			t.Errorf("expected validation error, got: %v", err)
		}
	})

	t.Run("no validator set", func(t *testing.T) {
		SetValidator(nil)

		envs := map[string]string{"NAME": "test"}
		var cfg Config
		err := Unmarshal(envs, &cfg)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestEmptyValueHandling(t *testing.T) {
	type Config struct {
		Name    string `env:"NAME"`
		Count   int    `env:"COUNT"`
		Enabled bool   `env:"ENABLED"`
	}

	envs := map[string]string{"NAME": "test"}
	var cfg Config
	err := Unmarshal(envs, &cfg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "test" {
		t.Errorf("Name = %v, want test", cfg.Name)
	}
	if cfg.Count != 0 {
		t.Errorf("Count = %v, want 0 (zero value)", cfg.Count)
	}
	if cfg.Enabled != false {
		t.Errorf("Enabled = %v, want false (zero value)", cfg.Enabled)
	}
}

func TestUnmarshalFromFileOnly(t *testing.T) {
	content := "FILE_KEY=file_value\n"
	f, err := os.CreateTemp("", "env")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(content)
	f.Close()

	os.Setenv("SYSTEM_KEY", "system_value")
	defer os.Unsetenv("SYSTEM_KEY")

	type Config struct {
		FileKey   string `env:"FILE_KEY"`
		SystemKey string `env:"SYSTEM_KEY"`
	}

	var cfg Config
	if err := UnmarshalFromFileOnly(f.Name(), &cfg); err != nil {
		t.Fatalf("UnmarshalFromFileOnly() error = %v", err)
	}

	if cfg.FileKey != "file_value" {
		t.Errorf("FileKey = %v, want file_value", cfg.FileKey)
	}
	if cfg.SystemKey != "" {
		t.Errorf("SystemKey = %v, want empty (should not read system env)", cfg.SystemKey)
	}
}

type threadSafeMockValidator struct{}

func (m *threadSafeMockValidator) Struct(v interface{}) error {
	return nil
}

func TestValidatorThreadSafety(t *testing.T) {
	defer SetValidator(nil)

	type Config struct {
		Name string `env:"NAME"`
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				SetValidator(&threadSafeMockValidator{})
				SetValidator(nil)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				envs := map[string]string{"NAME": "test"}
				var cfg Config
				_ = Unmarshal(envs, &cfg)
			}
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}
