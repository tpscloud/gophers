# A repeated flag splits values on commas

A Go CLI built with Cobra v1.10.2 has `app notify --message <text>`, which
may be repeated to send several messages. It is defined with
`StringSliceVar`. Users report that `app notify --message "Hello, world"`
sends two messages, "Hello" and " world".

Fix the flag so each `--message` value is kept whole, and add a test.
