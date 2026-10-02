// New responses provide actor-specific decisions; legacy responses retain the
// original designated-recipient rule rather than inferring company authority.
export function workReportReviewCapabilities({ current, ready, status, capabilities, canReview, canManage, recipientID, userID }: {
  current: boolean; ready: boolean; status?: string;
  capabilities?: { can_acknowledge: boolean; can_request_changes: boolean; can_discuss: boolean };
  canReview: boolean; canManage: boolean; recipientID?: string; userID: string;
}) {
  if (!ready) return { acknowledge: false, requestChanges: false, discuss: false };
  const legacyReview = current && canReview && status === 'submitted' && (canManage || recipientID === userID);
  return {
    acknowledge: current && (capabilities ? capabilities.can_acknowledge : legacyReview),
    requestChanges: current && (capabilities ? capabilities.can_request_changes : legacyReview),
    discuss: capabilities?.can_discuss === true,
  };
}
