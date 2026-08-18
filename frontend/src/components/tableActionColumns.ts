export type ActionColumnCandidate = {
  key: string;
  action?: boolean;
};

export function isActionColumn(column: ActionColumnCandidate) {
  return column.action === true || /^actions?$/i.test(column.key.trim());
}

export function actionColumnsFirst<T extends ActionColumnCandidate>(columns: T[]) {
  const actions = columns.filter(isActionColumn);
  if (!actions.length) return columns;
  return [...actions, ...columns.filter((column) => !isActionColumn(column))];
}
