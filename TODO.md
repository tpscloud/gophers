# TODO

- [ ] wails3: in the Verify section, tell the agent to give the user the
  `wails3 generate bindings` (or Taskfile) command whenever a service method
  changes, not only to regenerate the bindings itself. The 0.1.1 evals lost a
  point on `wails3-service-errors` (`regenerate_bindings`): the generated
  bindings were correct but the handoff never named the command. The same
  scenario scored 100 on 0.1.0, so ship this with the next release rather than
  on its own; each release reruns every eval.
- [ ] Plan how skills follow upstream releases. Decide how a new Cobra, pflag,
  Viper, or Wails release is detected (for example a scheduled
  `go list -m -u` in each `examples/<skill>` module), who reviews the API diff,
  and the order: bump that module's pins, fix its behavior tests, re-verify the
  skill text, then bump the plugin version to publish. Wails v3 is a
  prerelease, so expect breaking changes between betas.
- [ ] Add skill metrics to CI, as the README promises. Record the skills.sh
  audits, the Tessl quality reviews, and the eval results with and without the
  skills per release, so SECURITY.md and the badges come from CI output rather
  than hand edits. Evals cost Tessl credits, so run them only when the plugin
  version changes.
