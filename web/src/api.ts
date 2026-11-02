// Typed client for the backend HTTP contract (see api/routes.py, api/schemas.py).
// Times are Unix seconds. Field names match the JSON exactly.

export interface Position {
  latitude: number | null
  longitude: number | null
  altitude: number | null
  relative_altitude: number | null
}

export interface Velocity {
  velocity_x: number | null
  velocity_y: number | null
  velocity_z: number | null
}

export interface VehicleState {
  system_id: number | null
  component_id: number | null
  vehicle_type: string | null
  autopilot: string | null
  is_armed: boolean | null
  flight_mode: string | null
  system_status: string | null
  heartbeat_count: number
  position: Position | null
  velocity: Velocity | null
  heading: number | null
}

export type CommandStatus =
  | 'PENDING'
  | 'ACHIEVED'
  | 'ACK_TIMEOUT'
  | 'COMMAND_REJECTED'
  | 'STATE_TIMEOUT'
  | 'ALREADY_PENDING'
  | 'FAILED'

export interface CommandRecord {
  id: string
  system_id: number
  cmd: { name: string }
  status: CommandStatus | string
  error: string | null
  created_at: number
  updated_at: number
}

export interface CommandAccepted {
  command_id: string
  status: string
}

export const isTerminal = (status: string): boolean => status !== 'PENDING'

// Two different kinds of failure, on purpose:
//  - ServerDown: no usable answer (network error, proxy 502/504, 5xx). Drives the "no connection" banner and backoff.
//  - ApiError:   the server answered with a contract error body {error, detail} (e.g. UNKNOWN_COMMAND).
export class ServerDown extends Error {}

export class ApiError extends Error {
  status: number
  code: string
  constructor(status: number, code: string, detail: string) {
    super(detail)
    this.status = status
    this.code = code
  }
}

async function request(path: string, init?: RequestInit): Promise<Response> {
  let res: Response
  try {
    res = await fetch(path, init)
  } catch (e) {
    throw new ServerDown(`cannot reach server: ${String(e)}`)
  }
  if (res.status >= 500) throw new ServerDown(`server error ${res.status}`)
  return res
}

async function toApiError(res: Response): Promise<ApiError> {
  // Contract errors are {error, detail}; FastAPI's 422 is {detail: [...]}. Be tolerant of both.
  const body = await res.json().catch(() => ({}))
  const code = typeof body.error === 'string' ? body.error : `HTTP_${res.status}`
  const detail = typeof body.detail === 'string' ? body.detail : JSON.stringify(body.detail ?? body)
  return new ApiError(res.status, code, detail)
}

/** null means the server is fine but has had no heartbeat from this vehicle yet (404 NO_VEHICLE). */
export async function getVehicle(systemId: number): Promise<VehicleState | null> {
  const res = await request(`/vehicles/${systemId}`)
  if (res.ok) return res.json()
  const err = await toApiError(res)
  if (err.code === 'NO_VEHICLE') return null
  throw err
}

export async function listCommands(limit = 50, systemId?: number): Promise<CommandRecord[]> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (systemId !== undefined) params.set('system_id', String(systemId))
  const res = await request(`/commands?${params}`)
  if (!res.ok) throw await toApiError(res)
  return res.json()
}

export async function getCommand(commandId: string): Promise<CommandRecord> {
  const res = await request(`/commands/${encodeURIComponent(commandId)}`)
  if (!res.ok) throw await toApiError(res)
  return res.json()
}

export async function getVehicles(): Promise<VehicleState[]>{
  const res = await request('/vehicles')
  if(res.ok && res!==undefined) return res.json()
  else
    throw await toApiError(res)
}

export async function postCommand(systemId:number, name:string, params?: Record<string, unknown>): Promise<CommandAccepted>{
  const init: RequestInit = {method: 'POST'}
  if(params !==undefined){
    init.headers = {'Content-Type': 'application/json'}
    init.body = JSON.stringify(params)
  }
  const res = await request(`/vehicles/${systemId}/commands/${encodeURIComponent(name)}`, init)
  if(!res.ok) throw await toApiError(res)
  return res.json()
}