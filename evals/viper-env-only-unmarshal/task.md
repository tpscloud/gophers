# An environment variable is missing after Unmarshal

A Go service uses Viper v1.21.0. Its loader calls:

```go
v := viper.New()
v.SetEnvPrefix("APP")
v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
v.AutomaticEnv()
v.SetDefault("db.host", "localhost")
var cfg Config // has DB.Host and DB.Password with mapstructure tags
err := v.Unmarshal(&cfg)
```

With `APP_DB_PASSWORD=secret` set, `v.GetString("db.password")` returns
`secret`, but `cfg.DB.Password` is empty. There is no default for the
password and it must never appear in the config file.

Explain why and fix the loader. Add a test.
