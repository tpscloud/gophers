# Rename a profile from the frontend

A Wails v3.0.0-beta.29 app registers `application.NewService(&ProfileService{})`.
Add a method the frontend calls to rename a saved profile. It takes a request
with the profile ID and the new name and returns the updated profile. Names
must be 1 to 64 characters after trimming and unique; IDs must exist. The
service also needs to load profiles from disk when the app starts. The UI
fires renames quickly enough that two can overlap. The frontend is
TypeScript and uses the generated bindings in `frontend/bindings`.

Write the Go method, its types, the startup loading, a Go test for the error
cases, and the frontend call that shows the error to the user.
