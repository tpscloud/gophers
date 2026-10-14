// Placeholder frontend. A bundled app imports the generated bindings from
// frontend/bindings and the runtime from @wailsio/runtime; this file uses the
// runtime the asset server serves at /wails/runtime.js, so it needs no build.
const root = document.getElementById("app");

try {
  const { Call, Events } = await import("/wails/runtime.js");
  const list = () => Call.ByName("github.com/tpscloud/gophers/examples/wails3/notes.Service.ListNotes");
  const show = (notes) => { root.textContent = `${notes.length} notes`; };
  list().then(show).catch((err) => { root.textContent = `Could not load notes: ${err.message}`; });
  Events.On("notes:changed", () => list().then(show));
} catch {
  root.textContent = "Open this page in the Wails app; the runtime is not available here.";
}
