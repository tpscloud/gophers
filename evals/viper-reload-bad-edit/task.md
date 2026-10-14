# A bad config edit reaches the running server

A long-running Go HTTP server uses Viper v1.21.0. At startup it decodes and
validates config into a struct. For live reloads it does:

```go
v.OnConfigChange(func(fsnotify.Event) {
	limit = v.GetInt("rate.limit") // read by request handlers
})
v.WatchConfig()
```

After an operator saved a file with a YAML syntax error, the server kept
running, but nobody noticed the edit was ignored. After another edit set
`rate.limit: 0`, every request was rejected. The race detector also flags
`limit`.

Make reloads safe: a bad edit must be reported and must not replace the
current settings, and handlers must read settings without a data race.
Add a test.
