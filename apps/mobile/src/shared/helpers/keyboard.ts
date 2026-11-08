/** The part of React Native's `Keyboard` this helper needs (injected, so it runs in node tests). */
export interface KeyboardLike {
  isVisible(): boolean;
  dismiss(): void;
  addListener(event: 'keyboardDidHide', listener: () => void): { remove(): void };
}

/** keyboardDidHide normally arrives in ~250 ms; past this the sheet presents anyway. */
export const KEYBOARD_HIDE_MAX_MS = 600;

/**
 * Dismisses the keyboard and resolves once it is gone (or after the cap), so a sheet does not
 * present while the amount field's keypad is still sliding down.
 */
export function waitForKeyboardHidden(keyboard: KeyboardLike): Promise<void> {
  if (!keyboard.isVisible()) return Promise.resolve();
  return new Promise((resolve) => {
    const finish = () => {
      clearTimeout(timer);
      subscription.remove();
      resolve();
    };
    const subscription = keyboard.addListener('keyboardDidHide', finish);
    const timer = setTimeout(finish, KEYBOARD_HIDE_MAX_MS);
    keyboard.dismiss();
  });
}
