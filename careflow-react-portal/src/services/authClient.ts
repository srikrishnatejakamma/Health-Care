import { UserManager, WebStorageStateStore } from 'oidc-client-ts';
const authority=import.meta.env.VITE_OIDC_AUTHORITY as string|undefined;
const clientId=import.meta.env.VITE_OIDC_CLIENT_ID as string|undefined;
export const oidcConfigured=Boolean(authority&&clientId);
// Tokens are kept in memory. sessionStorage stores only the temporary PKCE state.
const memoryStore={length:0,clear(){this.length=0},getItem(_key:string){return null},key(_index:number){return null},removeItem(_key:string){},setItem(_key:string,_value:string){}};
export const oidc=oidcConfigured?new UserManager({authority:authority!,client_id:clientId!,redirect_uri:`${location.origin}/auth/callback`,post_logout_redirect_uri:location.origin,response_type:'code',scope:import.meta.env.VITE_OIDC_SCOPE||'openid profile',automaticSilentRenew:false,userStore:new WebStorageStateStore({store:memoryStore}),stateStore:new WebStorageStateStore({store:sessionStorage})}):null;
