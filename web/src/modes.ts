export const MODES = ['STABILIZE', 'GUIDED', 'LOITER', 'RTL', 'LAND'] as const
export type FlightMode = (typeof MODES)[number]

export const TAKEOFF_MIN_ALT = 2
export const TAKEOFF_MAX_ALT = 100

// Keep in step with TAKEOFF_MIN_ALT / MAX_ALT in alpha/application/command_params.py.
export const GOTO_MIN_ALT = 2
export const GOTO_MAX_ALT = 400

