export function workItemObserverReadOnly({ explicitReadOnly, hasCompanySupervision, detailFresh, canAct, canDecide }: {
  explicitReadOnly: boolean; hasCompanySupervision: boolean; detailFresh: boolean; canAct?: boolean; canDecide?: boolean;
}) {
  return explicitReadOnly || (hasCompanySupervision && (!detailFresh || (canAct !== true && canDecide !== true)));
}
