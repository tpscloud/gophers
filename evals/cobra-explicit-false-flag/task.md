# An explicit false flag is ignored

A Go CLI built with Cobra v1.10.2 has `app deploy --notify`, a bool flag that
defaults to false. A config file can also set `notify: true`. Users report that
`app deploy --notify=false` still sends a notification when the config file
says true. The flag must win when the user typed it.

Fix the command and add a test.
