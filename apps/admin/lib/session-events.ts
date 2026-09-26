export function signalLogout() {
  // Revocation contains no credential or order data; same-tab listeners clear synchronously.
  window.dispatchEvent(new Event("commerce-session-logout"));
  try {
    const channel = new BroadcastChannel("commerce-session");
    channel.postMessage({ type: "logout" });
    channel.close();
  } catch {
    /* storage event remains available */
  }
  try {
    localStorage.setItem("commerce-session-logout", crypto.randomUUID());
  } catch {
    /* other tabs also check their cookie on focus */
  }
}
