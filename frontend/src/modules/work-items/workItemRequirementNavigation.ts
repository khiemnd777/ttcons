const highlightClasses = ['ring-2', 'ring-blue-400', 'ring-offset-2'] as const;

export function revealWorkItemRequirementTarget(
  container: HTMLElement | null,
  focusTarget?: HTMLElement | null,
) {
  if (!container) return undefined;

  const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
  container.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth', block: 'start' });
  container.classList.add(...highlightClasses);

  const frame = window.requestAnimationFrame(() => {
    (focusTarget ?? container).focus({ preventScroll: true });
  });
  const timer = window.setTimeout(() => {
    container.classList.remove(...highlightClasses);
  }, 1800);

  return () => {
    window.cancelAnimationFrame(frame);
    window.clearTimeout(timer);
    container.classList.remove(...highlightClasses);
  };
}

export function firstFocusableRequirementControl(container: HTMLElement | null) {
  if (!container) return null;
  return container.querySelector<HTMLElement>(
    'input:not([disabled]), textarea:not([disabled]), select:not([disabled]), button:not([disabled]), a[href]',
  );
}
