# A child hook skips root config loading

A Go CLI built with Cobra v1.10.2 loads its config file in the root command's
`PersistentPreRunE`. After someone added a `PersistentPreRunE` to the `sync`
subcommand to open a lock file, `app sync` started running with an empty
config. Every other subcommand still works.

Explain the cause and fix it so `sync` gets both the config and the lock.
