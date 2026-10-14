# The wrong config file gets parsed

A Go CLI uses Viper v1.21.0 and reads an optional YAML file:

```go
v.SetConfigName("app")
v.SetConfigType("yaml")
v.AddConfigPath(".")
v.AddConfigPath(userConfigDir)
if cfgFile != "" {
	v.SetConfigFile(cfgFile)
}
_ = v.ReadInConfig() // the file is optional
```

Two bug reports: in a project directory that also contains another tool's
`app.toml`, the CLI ignores the user's `~/.config/app/app.yaml` and its
settings act strangely; and `--config /typo/path.yaml` runs with defaults
instead of failing.

Explain both and fix the loading code so the file stays optional but a
`--config` path that does not exist is an error. Add tests.
