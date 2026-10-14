# A flag default overrides the config file

A Go CLI uses Cobra v1.10.2 and Viper v1.21.0. The `serve` command defines
`--port` with a default of 8080. Before loading config, its `RunE` does:

```go
port, _ := cmd.Flags().GetInt("port")
v.Set("server.port", port)
```

The config file says `server.port: 9000` and operators also set
`APP_SERVER_PORT=9100` in some environments, but the server always listens
on 8080 unless `--port` is passed. The intended order is: typed flag, then
environment, then file, then the default.

Fix the loading code and add tests for each step of that order.
