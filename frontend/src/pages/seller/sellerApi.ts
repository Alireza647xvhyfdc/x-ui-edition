import { HttpUtil } from '@/utils';

export interface SellerInfo {
  id: number;
  username: string;
  expire: number;
  totalGB: number;
  maxClients: number;
  inboundIds: number[];
  clientCount: number;
  usedBytes: number;
  blocked: boolean;
}

export interface InboundOption {
  id: number;
  remark: string;
  port: number;
  protocol: string;
}

export interface SellerClient {
  email: string;
  uuid: string;
  password: string;
  auth: string;
  flow: string;
  security: string;
  subId: string;
  enable: boolean;
  totalGB: number;
  expiryTime: number;
  limitIp: number;
  tgId: number;
  comment: string;
  inboundIds: number[];
  traffic?: { up: number; down: number };
}

export const BYTES_PER_GB = 1024 ** 3;

// Throws with the server's message so each caller can show it in a toast.
export async function postSeller<T = unknown>(url: string, body?: unknown): Promise<T> {
  const msg = await HttpUtil.post<T>(url, body);
  if (!msg?.success) throw new Error(msg?.msg || 'Request failed');
  return msg.obj as T;
}

export async function getSeller<T>(url: string): Promise<T> {
  const msg = await HttpUtil.get<T>(url, undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Request failed');
  return msg.obj as T;
}

export const fetchSellerInfo = () => getSeller<SellerInfo>('/panel/api/sellers/me');
export const fetchSellerClients = () => getSeller<SellerClient[]>('/panel/api/clients/list');
export const fetchInboundOptions = () => getSeller<InboundOption[]>('/panel/api/inbounds/list/slim');

export function expiryLabel(expire: number, never: string): string {
  if (!expire) return never;
  const d = new Date(expire * 1000);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

// The client expiry is the end of the chosen local day, in milliseconds; empty means never.
export function dayToExpiryMs(day?: string): number {
  if (!day) return 0;
  return new Date(`${day}T23:59:59`).getTime();
}

export function expiryMsToDay(ms: number): string | undefined {
  if (!ms) return undefined;
  return expiryLabel(Math.floor(ms / 1000), '');
}
