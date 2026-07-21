export function validateSSELifetimeWindow(scenario, serverLifetimeSeconds, minJitterRatio, marginSeconds) {
  const testSeconds = scenarioDurationSeconds(scenario);
  const earliestTurnoverSeconds = serverLifetimeSeconds * minJitterRatio;
  if (earliestTurnoverSeconds <= testSeconds + marginSeconds) {
    throw new Error(`SSE server lifetime guard failed: earliest turnover ${earliestTurnoverSeconds}s must exceed test ${testSeconds}s plus margin ${marginSeconds}s`);
  }
  return { testSeconds, earliestTurnoverSeconds, marginSeconds };
}

export function scenarioDurationSeconds(scenario) {
  if (scenario.duration) return durationSeconds(scenario.duration);
  return (scenario.stages || []).reduce((total, stage) => total + durationSeconds(stage.duration), 0);
}

function durationSeconds(value) {
  const match = /^(\d+(?:\.\d+)?)(ms|s|m|h)$/.exec(String(value));
  if (!match) throw new Error(`Invalid scenario duration: ${value}`);
  const multipliers = { ms: 0.001, s: 1, m: 60, h: 3600 };
  return Number(match[1]) * multipliers[match[2]];
}
