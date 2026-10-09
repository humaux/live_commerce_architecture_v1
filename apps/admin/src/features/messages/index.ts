// Purpose: expose the existing private-view lifetime boundary for other admin features.
// Depends on: messages/use-privacy; implementation remains owned by the inbox feature.
// Used by: live comment stream; no second privacy implementation.
export { useInboxPrivacy } from "./use-privacy";
