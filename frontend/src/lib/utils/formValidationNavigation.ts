import { useCallback, useEffect, useRef, type RefObject } from 'react';

const validationHighlightClasses = [
  'ring-2',
  'ring-rose-400',
  'ring-offset-2',
  'transition-shadow',
] as const;

const focusableSelector = [
  'input:not([disabled])',
  'textarea:not([disabled])',
  'select:not([disabled])',
  'button:not([disabled])',
  'a[href]',
  '[tabindex]:not([tabindex="-1"])',
].join(', ');

export type ValidationNavigationIssue = {
  key?: string;
  prepare?: () => void;
  target?: HTMLElement | null | (() => HTMLElement | null);
};

export function orderedInvalidFieldKeys(
  errors: Record<string, unknown>,
  fieldOrder: readonly string[],
) {
  const keys = Object.keys(errors).filter((key) => Boolean(errors[key]));
  const known = new Set(fieldOrder);
  return [
    ...fieldOrder.filter((key) => Boolean(errors[key])),
    ...keys.filter((key) => !known.has(key)),
  ];
}

export function firstInvalidFieldKey(
  errors: Record<string, unknown>,
  fieldOrder: readonly string[] = [],
) {
  return orderedInvalidFieldKeys(errors, fieldOrder)[0];
}

export function findValidationTarget(root: HTMLElement | null, key: string) {
  if (!root || !key) return null;

  const keyedTargets = Array.from(root.querySelectorAll<HTMLElement>(
    '[data-validation-key], [data-workflow-field-key]',
  ));
  const matchingKeyedTargets = keyedTargets.filter((element) => (
    element.dataset.validationKey === key || element.dataset.workflowFieldKey === key
  ));
  const keyedTarget = matchingKeyedTargets.find((element) => element.getClientRects().length > 0)
    ?? matchingKeyedTargets[0];
  if (keyedTarget) return keyedTarget;

  const namedControls = Array.from(root.querySelectorAll<HTMLElement>('[name]'))
    .filter((element) => element.getAttribute('name') === key);
  const namedControl = namedControls.find((element) => element.getClientRects().length > 0)
    ?? namedControls[0];
  if (namedControl) return namedControl;

  return Array.from(root.querySelectorAll<HTMLElement>('[id]'))
    .find((element) => element.id === key) ?? null;
}

export function findFirstInvalidValidationTarget(root: HTMLElement | null) {
  if (!root) return null;
  const marked = root.querySelector<HTMLElement>('[data-validation-error="true"]');
  if (marked) return marked;
  const invalidControl = root.querySelector<HTMLElement>('[aria-invalid="true"]');
  return invalidControl?.closest<HTMLElement>('[data-validation-key], [data-workflow-field-key]')
    ?? invalidControl;
}

export function firstFocusableValidationControl(container: HTMLElement | null) {
  if (!container) return null;
  if (container.matches(focusableSelector)) return container;
  const invalidControls = Array.from(container.querySelectorAll<HTMLElement>('[aria-invalid="true"]'))
    .filter((control) => control.matches(focusableSelector));
  const controls = Array.from(container.querySelectorAll<HTMLElement>(focusableSelector));
  const candidates = [...invalidControls, ...controls.filter((control) => !invalidControls.includes(control))];
  return candidates.find((candidate) => candidate.getClientRects().length > 0) ?? candidates[0] ?? null;
}

export function revealValidationTarget(
  container: HTMLElement | null,
  focusTarget?: HTMLElement | null,
) {
  if (!container) return undefined;

  const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false;
  const addedHighlightClasses = validationHighlightClasses.filter((className) => (
    !container.classList.contains(className)
  ));
  container.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth', block: 'center' });
  container.classList.add(...addedHighlightClasses);

  const frame = window.requestAnimationFrame(() => {
    (focusTarget ?? firstFocusableValidationControl(container) ?? container).focus({ preventScroll: true });
  });
  const timer = window.setTimeout(() => {
    container.classList.remove(...addedHighlightClasses);
  }, 1800);

  return () => {
    window.cancelAnimationFrame(frame);
    window.clearTimeout(timer);
    container.classList.remove(...addedHighlightClasses);
  };
}

export function useValidationNavigation(rootRef: RefObject<HTMLElement | null>) {
  const cleanupRef = useRef<(() => void) | undefined>();

  useEffect(() => () => cleanupRef.current?.(), []);

  return useCallback((issue: string | ValidationNavigationIssue | undefined) => {
    cleanupRef.current?.();
    cleanupRef.current = undefined;
    if (!issue) return;

    const normalized = typeof issue === 'string' ? { key: issue } : issue;
    normalized.prepare?.();

    let secondFrame = 0;
    const firstFrame = window.requestAnimationFrame(() => {
      secondFrame = window.requestAnimationFrame(() => {
        const explicitTarget = typeof normalized.target === 'function'
          ? normalized.target()
          : normalized.target;
        const target = explicitTarget ?? (normalized.key
          ? findValidationTarget(rootRef.current, normalized.key)
          : findFirstInvalidValidationTarget(rootRef.current));
        cleanupRef.current = revealValidationTarget(
          target,
          firstFocusableValidationControl(target),
        );
      });
    });

    cleanupRef.current = () => {
      window.cancelAnimationFrame(firstFrame);
      if (secondFrame) window.cancelAnimationFrame(secondFrame);
    };
  }, [rootRef]);
}
