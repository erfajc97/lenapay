import { create } from 'zustand';

/**
 * Session flags only (M17): where the app is in the auth lifecycle. Tokens never live here
 * (they are in `shared/session`), and nothing here is persisted.
 *
 * - `booting`: reading the secure store at launch.
 * - `signedOut`: no session → auth screens.
 * - `locked`: a stored session exists but the person has not unlocked yet → biometric screen.
 * - `active`: unlocked, the app can call the API.
 */
type SessionStatus = 'booting' | 'signedOut' | 'locked' | 'active';

interface SessionState {
  status: SessionStatus;
  /** Email waiting for its verification code after signup. */
  pendingEmail: string | null;
  setStatus(status: SessionStatus): void;
  setPendingEmail(email: string | null): void;
}

export const useSessionStore = create<SessionState>()((set) => ({
  status: 'booting',
  pendingEmail: null,
  setStatus: (status) => set({ status }),
  setPendingEmail: (pendingEmail) => set({ pendingEmail }),
}));
