# Cleanup that never runs

A Go CLI built with Cobra v1.10.2 opens a database in the `import` command's
`PreRunE` and closes it in `PostRunE`. When the import fails, the database
lock file stays behind and the next run reports "database is locked".

Explain why and restructure the command so the database is always closed.
