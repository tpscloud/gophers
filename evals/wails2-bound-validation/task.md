# Rename a profile from the frontend

A Wails v2.16.0 app binds `*App`. Add a method the frontend calls to rename a
saved profile. It takes a request with the profile ID and the new name and
returns the updated profile. Names must be 1 to 64 characters after trimming
and unique; IDs must exist. The frontend is TypeScript and uses the generated
`wailsjs` bindings. Profiles live in a map on `App`, and the UI fires renames
quickly enough that two can overlap.

Write the Go method and its types, a Go test for the error cases, and the
frontend call that shows the error to the user.
