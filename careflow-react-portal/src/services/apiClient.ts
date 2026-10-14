import { QueryClient } from '@tanstack/react-query';
import { useSession } from '../stores/session';
import { ApiError, demoRequest } from './demoClient';
import type { Me } from '../shared/types';

export { ApiError };
export const queryClient=new QueryClient({defaultOptions:{queries:{staleTime:15_000,gcTime:60_000,retry:(count,error)=>!(error instanceof ApiError&&[401,403,404].includes(error.status))&&count<1,refetchOnWindowFocus:true},mutations:{retry:false}}});
const controllers=new Set<AbortController>();
export function clearSessionRequests(){controllers.forEach(c=>c.abort());controllers.clear();void queryClient.cancelQueries();queryClient.clear();}
export async function connect(token:string,tenant:string):Promise<Me>{const response=await fetch('/api/v1/me',{headers:{Authorization:`Bearer ${token.trim()}`,'X-Tenant-ID':tenant},cache:'no-store'});if(!response.ok){const error=await response.json().catch(()=>({}));throw new ApiError(response.status,error.error||'Unable to connect to the workspace.',error.correlation_id)}return response.json();}
export async function api<T>(path:string,options:{method?:string;body?:unknown;signal?:AbortSignal;key?:string}={}):Promise<T>{
  const session=useSession.getState();if(!session.me||session.mode==='signedout')throw new ApiError(401,'Please sign in to continue.');const {method='GET',body,signal,key=method==='GET'?'':crypto.randomUUID()}=options;
  if(session.mode==='demo'){if(signal?.aborted)throw new DOMException('Cancelled','AbortError');return demoRequest<T>(session.me,path,method,body,key)}
  const controller=new AbortController();controllers.add(controller);const abort=()=>controller.abort();signal?.addEventListener('abort',abort,{once:true});if(signal?.aborted)controller.abort();
  try{const response=await fetch(`/api/v1${path}`,{method,headers:{Authorization:`Bearer ${session.token}`,'X-Tenant-ID':session.tenantId,...(body?{'Content-Type':'application/json'}:{}),...(key?{'Idempotency-Key':key}:{})},body:body?JSON.stringify(body):undefined,signal:controller.signal,cache:'no-store'});if(useSession.getState().epoch!==session.epoch)throw new DOMException('Workspace changed','AbortError');if(!response.ok){const error=await response.json().catch(()=>({}));if(response.status===401){clearSessionRequests();useSession.getState().setConnection('signedout','',null)}throw new ApiError(response.status,error.error||'The request could not be completed.',error.correlation_id)}return response.json() as Promise<T>}
  finally{controllers.delete(controller);signal?.removeEventListener('abort',abort)}
}
export async function changeConnection(mode:'demo'|'live'|'signedout',token:string,me:Me|null){clearSessionRequests();useSession.getState().setConnection(mode,token,me)}
export const friendlyError=(error:unknown)=>{if(error instanceof ApiError){const messages:Record<string,string>={resource_conflict:'This record changed or conflicts with an existing reservation. Refresh and try again.',access_denied:'Your current role does not allow this action.',quota_exceeded:'This workspace has reached its request or processing limit. Please try again shortly.',tenant_inactive:'This workspace is suspended or inactive.',not_found:'This record is unavailable in your current workspace.',invalid_token:'Your session is invalid or expired. Please sign in again.'};return messages[error.message]||error.message}return error instanceof Error?error.message:'Something went wrong. Please try again.'};
