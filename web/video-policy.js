/* Same allowlist as video_links.go; the server is the authority. */
(()=>{
 const message='链接仅支持 HTTPS 的 Bilibili 或 YouTube 视频播放页；不接受其他网站、短链跳转或下载链接。B站短链请先打开，再复制完整视频地址。';
 const candidate=/(?:[a-z][a-z0-9+.-]{0,24}:\/\/|(?:https?|javascript|data|file|vbscript|blob|mailto):|\/\/|(?:[a-z0-9-]+\.)+[a-z]{2,63}\b)[^\s<>"'`，。；、（）【】]*/gi;
 function canonical(raw){
  if(!/^https:\/\//i.test(raw)||/\/\.{1,2}(?:\/|[?#]|$)/.test(raw))throw Error(message);
  const u=new URL(raw);let query=u.search.slice(1);if(query.includes(';'))throw Error(message);try{decodeURIComponent(query)}catch{throw Error(message)}if(u.protocol!=='https:'||u.username||u.password||u.port||/[\\\x00\r\n]/.test(raw))throw Error(message);
  // URL hides an explicit :443; reject ports consistently with the Go backend.
  if(/^https:\/\/[^/]+:\d+(?:\/|$)/i.test(raw))throw Error(message);
  for(const [k,v]of u.searchParams){let decoded=v;try{decoded=decodeURIComponent(v)}catch{}if(['url','redirect','redirect_url','redirect_uri','target','next','continue','link'].includes(k.toLowerCase())||/(?:https?|ftp|file|javascript|data):|\/\//i.test(v)||/(?:https?|ftp|file|javascript|data):|\/\//i.test(decoded))throw Error(message)}
  const host=u.hostname.toLowerCase(),path=u.pathname.replace(/\/$/,''),params=new URLSearchParams();let result='',platform='',id='';
  if(['bilibili.com','www.bilibili.com','m.bilibili.com'].includes(host)){
   id=path.startsWith('/video/')?path.slice(7):'';if(!/^(BV[A-Za-z0-9]{10}|av[1-9][0-9]{0,19})$/.test(id))throw Error(message);
   result='https://www.bilibili.com/video/'+id+'/';platform='Bilibili';
   const p=u.searchParams.getAll('p'),t=u.searchParams.getAll('t');if(p.length===1&&/^[1-9][0-9]{0,5}$/.test(p[0]))params.set('p',p[0]);if(t.length===1&&/^[0-9]{1,7}$/.test(t[0]))params.set('t',t[0]);
  }else if(['youtube.com','www.youtube.com','m.youtube.com','youtu.be'].includes(host)){
   if(host==='youtu.be')id=path.slice(1);else if(path==='/watch'&&u.searchParams.getAll('v').length===1)id=u.searchParams.get('v');else if(/^\/(shorts|live)\//.test(path))id=path.split('/')[2];
   if(!/^[A-Za-z0-9_-]{11}$/.test(id||'')||(/\/(shorts|live)\//.test(path)&&path.split('/').length!==3))throw Error(message);
   result='https://www.youtube.com/watch';platform='YouTube';params.set('v',id);const t=u.searchParams.getAll('t');if(t.length===1&&t[0]&&/^(?:[0-9]{1,7}|(?:[0-9]{1,3}h)?(?:[0-9]{1,3}m)?(?:[0-9]{1,3}s)?)$/.test(t[0]))params.set('t',t[0]);
  }else throw Error(message);
  params.sort();return {url:result+(params.size?'?'+params:''),platform,id};
 }
 function tokens(value){return Array.from(String(value).matchAll(new RegExp(candidate.source,'gi'))).filter(m=>m.index===0||value[m.index-1]!=='@').map(m=>({raw:m[0].replace(/[.,;!?:)\]}]+$/,''),index:m.index}))}
 function normalize(value){value=String(value);let result='',last=0;for(const token of tokens(value)){result+=value.slice(last,token.index)+canonical(token.raw).url;last=token.index+token.raw.length}return result+value.slice(last)}
 function prepare(body){if(!body||typeof body!=='object')return body;const out={...body};for(const key of ['player','account','community','reason','evidence','content','description','title','contact','website','response'])if(typeof out[key]==='string')out[key]=normalize(out[key]);return out}
 globalThis.VideoPolicy={canonical,tokens,normalize,prepare,message};
})();
