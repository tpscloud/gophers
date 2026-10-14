# TODO

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
