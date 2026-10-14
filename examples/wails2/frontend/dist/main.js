// Placeholder frontend. It calls the bound methods through the same global the
// generated wailsjs bindings use, and degrades when the bridge is missing so a
// plain browser shows a message instead of failing silently.
const api = window.go?.app?.App;
const root = document.getElementById("app");

if (!api) {
  root.textContent = "Open this page in the Wails app; the Go bridge is not available here.";
} else {
  api.ListNotes()
    .then((notes) => { root.textContent = `${notes.length} notes`; })
    .catch((err) => { root.textContent = `Could not load notes: ${err}`; });
}
