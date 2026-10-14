# TODO

- [ ] wails3: win back the Tessl quality points (97/100 on 0.1.2; actionability
  4 of 5). Either make the trimmed `main.go` block copy-paste runnable or link
  the full file under `examples/wails3/`, and show the `ServiceStartup` and
  `ServiceShutdown` signatures inline once in the service contract. The same
  text scored 100 on 0.1.0, so ship this with the next release, not on its own.
- [ ] Plan how skills follow upstream releases. Decide how a new Cobra, pflag,
  Viper, or Wails release is detected (for example a scheduled
  `go list -m -u` in each `examples/<skill>` module, plus
  `go list -m -versions` for wails3, because `-u` does not offer a newer
  prerelease to a module pinned to one), who reviews the API diff,
  and the order: bump that module's pins, fix its behavior tests, re-verify the
  skill text, then bump the plugin version to publish. Wails v3 is a
  prerelease, so expect breaking changes between betas.
- [ ] Add skill metrics to CI, as the README promises. Record the skills.sh
  audits, the Tessl quality reviews, and the eval results with and without the
  skills per release, so SECURITY.md and the badges come from CI output rather
  than hand edits. Evals cost Tessl credits, so run them only when the plugin
  version changes.
