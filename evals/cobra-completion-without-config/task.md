# Tab completion fails without a config file

A Go CLI built with Cobra v1.10.2 loads `~/.config/app/config.yaml` in the root
command's `PersistentPreRunE` and returns an error when the file is missing.
On a fresh machine, `app --help` works, but tab completion prints nothing and
`app completion zsh` fails with "config file not found".

Explain why and fix it without dropping the config requirement for real
commands.
