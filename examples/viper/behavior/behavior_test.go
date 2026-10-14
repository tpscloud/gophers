// Package behavior checks the Viper v1.21.0 behavior that skills/viper states
// as fact. When a test here fails after an upgrade, update the skill text
// before changing the test.
package behavior

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func unset(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
}

func read(t *testing.T, v *viper.Viper, path string) {
	t.Helper()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
}

func portFlag(t *testing.T, args ...string) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.Int("port", 7000, "")
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	return fs
}

// SKILL.md precedence: Set > changed flag > env > file > SetDefault > unchanged flag.
func TestPrecedence(t *testing.T) {
	file := write(t, t.TempDir(), "c.yaml", "port: 3\n")
	layers := func(changed bool, env, inFile, def, set bool) int {
		unset(t, "PORT")
		v := viper.New()
		args := []string{}
		if changed {
			args = append(args, "--port=1")
		}
		if err := v.BindPFlag("port", portFlag(t, args...).Lookup("port")); err != nil {
			t.Fatal(err)
		}
		if err := v.BindEnv("port"); err != nil {
			t.Fatal(err)
		}
		if env {
			t.Setenv("PORT", "2")
		}
		if inFile {
			read(t, v, file)
		}
		if def {
			v.SetDefault("port", 4)
		}
		if set {
			v.Set("port", 0)
		}
		return v.GetInt("port")
	}
	cases := []struct {
		name                         string
		changed, env, file, def, set bool
		want                         int
	}{
		{"Set wins", true, true, true, true, true, 0},
		{"changed flag beats env", true, true, true, true, false, 1},
		{"env beats file", false, true, true, true, false, 2},
		{"file beats default", false, false, true, true, false, 3},
		{"default beats unchanged flag", false, false, false, true, false, 4},
		{"unchanged flag is the last fallback", false, false, false, false, false, 7000},
	}
	for _, c := range cases {
		if got := layers(c.changed, c.env, c.file, c.def, c.set); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

// SKILL.md: copying parsed flag values into Set makes omitted flags beat env and files.
func TestSetFromFlagsBreaksPrecedence(t *testing.T) {
	t.Setenv("PORT", "2")
	v := viper.New()
	if err := v.BindEnv("port"); err != nil {
		t.Fatal(err)
	}
	fs := portFlag(t) // --port not given
	n, _ := fs.GetInt("port")
	v.Set("port", n)
	if got := v.GetInt("port"); got != 7000 {
		t.Fatalf("got %d", got)
	}
}

// SKILL.md: an explicit false flag is a changed flag and wins.
func TestExplicitFalseFlag(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.Bool("tls", false, "")
	if err := fs.Parse([]string{"--tls=false"}); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	v.SetDefault("tls", true)
	if err := v.BindPFlag("tls", fs.Lookup("tls")); err != nil {
		t.Fatal(err)
	}
	if v.GetBool("tls") {
		t.Fatal("explicit --tls=false lost to the default")
	}
}

// SKILL.md: Viper does not write env or file values back into flag variables.
func TestFlagVariableNotUpdated(t *testing.T) {
	t.Setenv("NAME", "from-env")
	var name string
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.StringVar(&name, "name", "flag-default", "")
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	if err := v.BindPFlag("name", fs.Lookup("name")); err != nil {
		t.Fatal(err)
	}
	if err := v.BindEnv("name"); err != nil {
		t.Fatal(err)
	}
	if v.GetString("name") != "from-env" || name != "flag-default" {
		t.Fatalf("viper=%q variable=%q", v.GetString("name"), name)
	}
}

// SKILL.md: BindPFlag rejects a nil flag, which is what Lookup returns for a typo.
func TestBindPFlagNil(t *testing.T) {
	v := viper.New()
	if err := v.BindPFlag("port", pflag.NewFlagSet("t", pflag.ContinueOnError).Lookup("prot")); err == nil {
		t.Fatal("no error for a nil flag")
	}
}

// SKILL.md: AutomaticEnv answers Get but not Unmarshal for a key Viper has
// never seen; BindEnv or SetDefault registers it.
func TestAutomaticEnvUnmarshalGap(t *testing.T) {
	t.Setenv("APP_TOKEN", "secret")
	type cfg struct {
		Token string `mapstructure:"token"`
	}
	v := viper.New()
	v.SetEnvPrefix("APP")
	v.AutomaticEnv()
	var c cfg
	if err := v.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if v.GetString("token") != "secret" || c.Token != "" || len(v.AllKeys()) != 0 {
		t.Fatalf("get=%q unmarshal=%q keys=%v", v.GetString("token"), c.Token, v.AllKeys())
	}

	if err := v.BindEnv("token"); err != nil {
		t.Fatal(err)
	}
	if err := v.Unmarshal(&c); err != nil || c.Token != "secret" {
		t.Fatalf("after BindEnv: %q %v", c.Token, err)
	}

	w := viper.New()
	w.SetEnvPrefix("APP")
	w.AutomaticEnv()
	w.SetDefault("token", "")
	var d cfg
	if err := w.Unmarshal(&d); err != nil || d.Token != "secret" {
		t.Fatalf("after SetDefault: %q %v", d.Token, err)
	}
}

// SKILL.md: BindEnv(key) derives PREFIX_KEY, upper-cased, and the key replacer
// applies; BindEnv(key, name) uses the name as given, without the prefix.
func TestBindEnvNames(t *testing.T) {
	unset(t, "APP_DB_PASSWORD", "DATABASE_PASSWORD")
	t.Setenv("APP_SERVER_READ_TIMEOUT", "2s")
	t.Setenv("DATABASE_PASSWORD", "exact")
	t.Setenv("APP_DATABASE_PASSWORD", "prefixed")
	v := viper.New()
	v.SetEnvPrefix("app")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	if err := v.BindEnv("server.read-timeout"); err != nil {
		t.Fatal(err)
	}
	if err := v.BindEnv("db.password", "DATABASE_PASSWORD"); err != nil {
		t.Fatal(err)
	}
	if v.GetDuration("server.read-timeout") != 2*time.Second || v.GetString("db.password") != "exact" {
		t.Fatalf("timeout=%v password=%q", v.Get("server.read-timeout"), v.GetString("db.password"))
	}
}

// SKILL.md: an empty variable counts as unset unless AllowEmptyEnv(true).
func TestEmptyEnv(t *testing.T) {
	t.Setenv("LEVEL", "")
	v := viper.New()
	v.SetDefault("level", "info")
	if err := v.BindEnv("level"); err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("level"); got != "info" {
		t.Fatalf("got %q", got)
	}
	v.AllowEmptyEnv(true)
	if got := v.GetString("level"); got != "" {
		t.Fatalf("with AllowEmptyEnv: got %q", got)
	}
}

// SKILL.md: keys are case-insensitive, and so are map keys read from a file.
func TestKeysAreCaseInsensitive(t *testing.T) {
	v := viper.New()
	read(t, v, write(t, t.TempDir(), "c.yaml", "Server:\n  Port: 1\nlabels:\n  Team: Core\n"))
	if v.GetInt("server.port") != 1 || v.GetInt("SERVER.PORT") != 1 {
		t.Fatal("lookup is case-sensitive")
	}
	labels := v.GetStringMapString("labels")
	if _, ok := labels["team"]; !ok || labels["team"] != "Core" {
		t.Fatalf("labels=%v; keys are lowered, values are kept", labels)
	}
}

// SKILL.md: Viper's discovery returns ConfigFileNotFoundError on a miss; an
// explicit SetConfigFile that is missing returns a plain not-exist error.
func TestNotFoundErrors(t *testing.T) {
	v := viper.New()
	v.SetConfigName("app")
	v.AddConfigPath(t.TempDir())
	err := v.ReadInConfig()
	if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); !ok {
		t.Fatalf("discovery miss: %T %v", err, err)
	}
	var notFound viper.ConfigFileNotFoundError // value target, for Go before 1.26
	if !errors.As(err, &notFound) {
		t.Fatal("errors.As with a value target did not match")
	}
	var ptr *viper.ConfigFileNotFoundError
	if errors.As(err, &ptr) {
		t.Fatal("a pointer target matched; the skill says it never does")
	}

	w := viper.New()
	w.SetConfigFile(filepath.Join(t.TempDir(), "app.yaml"))
	err = w.ReadInConfig()
	if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); ok || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("explicit miss: %T %v", err, err)
	}
}

// SKILL.md: discovery tries extensions in SupportedExts order (json before
// yaml), and SetConfigType forces the parser for whatever it finds.
func TestDiscoveryOrderAndForcedType(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app.yaml", "port: 1\n")
	write(t, dir, "app.json", `{"port": 2}`)
	v := viper.New()
	v.SetConfigName("app")
	v.AddConfigPath(dir)
	if err := v.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	if v.GetInt("port") != 2 || filepath.Base(v.ConfigFileUsed()) != "app.json" {
		t.Fatalf("port=%d used=%s", v.GetInt("port"), v.ConfigFileUsed())
	}

	dir = t.TempDir()
	write(t, dir, "app.yaml", "port: 1\n")
	write(t, dir, "app.toml", "port = 2\n")
	w := viper.New()
	w.SetConfigName("app")
	w.SetConfigType("yaml")
	w.AddConfigPath(dir)
	err := w.ReadInConfig()
	if err == nil || filepath.Base(w.ConfigFileUsed()) != "app.toml" {
		t.Fatalf("err=%v used=%s; expected app.toml parsed as YAML", err, w.ConfigFileUsed())
	}
}

// SKILL.md: SetConfigType also overrides the extension of an explicit file,
// and an explicit file with no extension needs it.
func TestConfigTypeAndExplicitFile(t *testing.T) {
	dir := t.TempDir()
	bare := write(t, dir, "appconfig", "port: 1\n")
	v := viper.New()
	v.SetConfigFile(bare)
	err := v.ReadInConfig()
	if _, ok := errors.AsType[viper.UnsupportedConfigError](err); !ok {
		t.Fatalf("no extension: %T %v", err, err)
	}
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil || v.GetInt("port") != 1 {
		t.Fatalf("with SetConfigType: %v", err)
	}

	w := viper.New()
	w.SetConfigType("yaml")
	w.SetConfigFile(write(t, dir, "app.toml", "[server]\nport = 2\n"))
	if err := w.ReadInConfig(); err == nil {
		t.Fatal("a TOML file parsed as YAML without error")
	}
}

// SKILL.md: the core decodes YAML, JSON, TOML, and dotenv only; other
// extensions in SupportedExts fail to decode.
func TestCoreFormats(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"a.yaml": "k: v\n", "a.yml": "k: v\n", "a.json": `{"k": "v"}`,
		"a.toml": "k = \"v\"\n", "a.env": "K=v\n", "a.dotenv": "K=v\n",
	} {
		v := viper.New()
		read(t, v, write(t, dir, name, body))
		if v.GetString("k") != "v" {
			t.Errorf("%s: k=%q", name, v.GetString("k"))
		}
	}
	for _, name := range []string{"a.ini", "a.hcl", "a.properties"} {
		v := viper.New()
		v.SetConfigFile(write(t, dir, name, "k = v\n"))
		err := v.ReadInConfig()
		if err == nil || !strings.Contains(err.Error(), "decoder not found") {
			t.Errorf("%s: err=%v", name, err)
		}
	}
}

// SKILL.md: Unmarshal ignores unknown keys; UnmarshalExact rejects them.
func TestUnmarshalExact(t *testing.T) {
	v := viper.New()
	read(t, v, write(t, t.TempDir(), "c.yaml", "port: 1\nprot: 2\n"))
	var c struct {
		Port int `mapstructure:"port"`
	}
	if err := v.Unmarshal(&c); err != nil || c.Port != 1 {
		t.Fatalf("Unmarshal: %v", err)
	}
	if err := v.UnmarshalExact(&c); err == nil || !strings.Contains(err.Error(), "prot") {
		t.Fatalf("UnmarshalExact: %v", err)
	}
}

// SKILL.md: BindPFlags binds every flag, so an operational flag becomes an
// unknown key for UnmarshalExact.
func TestBindPFlagsPollutesClosedSchema(t *testing.T) {
	fs := portFlag(t)
	fs.Bool("json", false, "")
	v := viper.New()
	if err := v.BindPFlags(fs); err != nil {
		t.Fatal(err)
	}
	var c struct {
		Port int `mapstructure:"port"`
	}
	if err := v.UnmarshalExact(&c); err == nil || !strings.Contains(err.Error(), "json") {
		t.Fatalf("err=%v", err)
	}
}

// SKILL.md: a getter returns zero for a malformed value; Unmarshal reports it.
func TestGetterHidesBadValue(t *testing.T) {
	t.Setenv("PORT", "eighty")
	v := viper.New()
	if err := v.BindEnv("port"); err != nil {
		t.Fatal(err)
	}
	if v.GetInt("port") != 0 {
		t.Fatal("GetInt did not return zero")
	}
	var c struct {
		Port int `mapstructure:"port"`
	}
	if err := v.Unmarshal(&c); err == nil {
		t.Fatal("Unmarshal accepted eighty")
	}
}

// SKILL.md: Unmarshal splits an env string on commas only; GetStringSlice
// splits on whitespace.
func TestEnvSliceSplitting(t *testing.T) {
	t.Setenv("HOSTS", "a,b c")
	v := viper.New()
	if err := v.BindEnv("hosts"); err != nil {
		t.Fatal(err)
	}
	var c struct {
		Hosts []string `mapstructure:"hosts"`
	}
	if err := v.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if got := v.GetStringSlice("hosts"); !slices.Equal(got, []string{"a,b", "c"}) {
		t.Fatalf("GetStringSlice=%q", got)
	}
	if !slices.Equal(c.Hosts, []string{"a", "b c"}) {
		t.Fatalf("Unmarshal=%q", c.Hosts)
	}
}

// SKILL.md: viper.DecodeHook replaces the default hooks, so durations stop
// decoding unless the default hooks are composed back in.
func TestDecodeHookReplacesDefaults(t *testing.T) {
	v := viper.New()
	v.SetDefault("timeout", "5s")
	var c struct {
		Timeout time.Duration `mapstructure:"timeout"`
	}
	custom := mapstructure.DecodeHookFuncType(func(_, _ reflect.Type, data any) (any, error) {
		return data, nil // stands in for an application-specific conversion
	})
	if err := v.Unmarshal(&c, viper.DecodeHook(custom)); err == nil {
		t.Fatal("a custom hook alone still decoded a duration string")
	}
	composed := mapstructure.ComposeDecodeHookFunc(
		custom,
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	)
	if err := v.Unmarshal(&c, viper.DecodeHook(composed)); err != nil || c.Timeout != 5*time.Second {
		t.Fatalf("composed: %v %v", c.Timeout, err)
	}
}

// references/reload-and-write.md: Sub returns nil for a missing or scalar key.
func TestSubNil(t *testing.T) {
	v := viper.New()
	v.SetDefault("server.port", 1)
	v.SetDefault("name", "x")
	if v.Sub("missing") != nil || v.Sub("name") != nil || v.Sub("server") == nil {
		t.Fatal("Sub nil cases changed")
	}
}

// references/reload-and-write.md: WriteConfigAs writes every effective
// setting, including env values and defaults; SafeWriteConfigAs refuses to
// overwrite.
func TestWriteMaterializesEffectiveSettings(t *testing.T) {
	t.Setenv("TOKEN", "secret")
	dir := t.TempDir()
	v := viper.New()
	read(t, v, write(t, dir, "app.yaml", "port: 1\n"))
	v.SetDefault("level", "info")
	if err := v.BindEnv("token"); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.yaml")
	if err := v.WriteConfigAs(out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"port: 1", "level: info", "token: secret"} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("written file lacks %q:\n%s", want, b)
		}
	}
	var buf bytes.Buffer
	if err := v.WriteConfigTo(&buf); err != nil || !bytes.Equal(buf.Bytes(), b) {
		t.Fatalf("WriteConfigTo differs: %v\n%s", err, buf.String())
	}
	err = v.SafeWriteConfigAs(out)
	if _, ok := errors.AsType[viper.ConfigFileAlreadyExistsError](err); !ok {
		t.Fatalf("SafeWriteConfigAs: %T %v", err, err)
	}
}

// references/reload-and-write.md: WriteConfig drops comments.
func TestWriteDropsComments(t *testing.T) {
	path := write(t, t.TempDir(), "app.yaml", "# keep me\nport: 1\n")
	v := viper.New()
	read(t, v, path)
	if err := v.WriteConfig(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("keep me")) {
		t.Fatalf("comment survived:\n%s", b)
	}
}

// references/reload-and-write.md: IsSet is true for a default-only key, and
// InConfig only looks at the file.
func TestIsSetAndInConfig(t *testing.T) {
	v := viper.New()
	read(t, v, write(t, t.TempDir(), "c.yaml", "port: 1\n"))
	v.SetDefault("level", "info")
	if !v.IsSet("level") || v.InConfig("level") || !v.InConfig("port") {
		t.Fatalf("IsSet(level)=%v InConfig(level)=%v InConfig(port)=%v",
			v.IsSet("level"), v.InConfig("level"), v.InConfig("port"))
	}
}

// SKILL.md: separate instances share nothing.
func TestInstancesAreIndependent(t *testing.T) {
	a, b := viper.New(), viper.New()
	a.Set("port", 1)
	if b.IsSet("port") || viper.IsSet("port") {
		t.Fatal("state leaked between instances")
	}
}
