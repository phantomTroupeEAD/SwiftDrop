import { useEffect } from 'react';

/**
 * Dialogs, menus and toasts render in a portal attached to <body>, outside the screen's
 * `data-role` wrapper, so the role palette from index.css would not reach them and they would
 * show the default blue. While a role-scoped screen is mounted, also set the role on <body>
 * (AGENTS.md Rule 6 asks for data-role on the root wrapper; this extends it to portals).
 */
export function useBodyRole(role: string): void {
  useEffect(() => {
    const previous = document.body.getAttribute('data-role');
    document.body.setAttribute('data-role', role);
    return () => {
      if (previous === null) document.body.removeAttribute('data-role');
      else document.body.setAttribute('data-role', previous);
    };
  }, [role]);
}
