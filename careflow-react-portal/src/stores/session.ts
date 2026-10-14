import { create } from 'zustand';
import { demoMe } from '../services/demoData';
import type { Me, Role } from '../shared/types';

export type Mode = 'demo' | 'live' | 'signedout';
interface Session { mode: Mode; tenantId: string; token: string; me: Me | null; epoch: string; facility: string; theme: 'light'|'dark'; maskPhi: boolean; sidebarCollapsed:boolean; notice:string; setConnection:(mode:Mode,token:string,me:Me|null)=>void; setFacility:(id:string)=>void; setTheme:()=>void; toggleMask:()=>void; toggleSidebar:()=>void; notify:(message:string)=>void; demoRole:(role:Role)=>void; }
const preference=(key:string,fallback:string)=>{try{return localStorage.getItem(key)||fallback}catch{return fallback}};
export const useSession=create<Session>((set,get)=>({
  mode:'demo',tenantId:'north-health',token:'',me:demoMe(),epoch:'demo-initial',facility:'all',
  theme:preference('careflow-theme','light')==='dark'?'dark':'light',maskPhi:false,sidebarCollapsed:false,notice:'',
  setConnection:(mode,token,me)=>set({mode,token,me,tenantId:me?.tenant.id||'',epoch:crypto.randomUUID(),facility:'all',maskPhi:false}),
  setFacility:facility=>set({facility}),setTheme:()=>{const theme=get().theme==='light'?'dark':'light';try{localStorage.setItem('careflow-theme',theme)}catch{/* Storage is optional. */}set({theme})},
  toggleMask:()=>set({maskPhi:!get().maskPhi}),toggleSidebar:()=>set({sidebarCollapsed:!get().sidebarCollapsed}),notify:notice=>set({notice}),
  demoRole:role=>{const me=demoMe(get().tenantId,role);get().setConnection('demo','',me)},
}));
