const HomeMotion=(()=>{
  const roots=[...document.querySelectorAll('[data-home-motion]')];
  const preference=matchMedia('(prefers-reduced-motion: reduce)');
  const visible=new Map(roots.map(root=>[root,false]));
  const control=document.getElementById('motion-toggle');
  let paused=false;
  function update(){
    for(const root of roots){
      const hoverStop=root.hasAttribute('data-hover-pause')&&root.matches(':hover,:focus-within');
      const stopped=paused||preference.matches||document.hidden||!visible.get(root)||hoverStop;
      root.classList.toggle('motion-paused',stopped);
      root.classList.toggle('motion-reduced',preference.matches);
      for(const svg of root.querySelectorAll('svg.network-svg')){
        if(stopped||!svg.getBoundingClientRect().width)svg.pauseAnimations();else svg.unpauseAnimations();
      }
    }
    if(control){control.disabled=preference.matches;control.textContent=preference.matches?'已减少动画':paused?'继续动效':'暂停动效';control.setAttribute('aria-pressed',String(paused));}
  }
  const observer=new IntersectionObserver(entries=>{for(const entry of entries)visible.set(entry.target,entry.isIntersecting);update()},{rootMargin:'80px',threshold:.02});
  for(const root of roots){observer.observe(root);root.addEventListener('pointerenter',update);root.addEventListener('pointerleave',update);root.addEventListener('focusin',update);root.addEventListener('focusout',()=>queueMicrotask(update));}
  document.addEventListener('visibilitychange',update);
  preference.addEventListener('change',update);
  window.addEventListener('resize',update,{passive:true});
  if(control)control.onclick=()=>{paused=!paused;update()};
  function duplicateLoop(container,items,makeCard){
    container.replaceChildren();const track=document.createElement('div');track.className='motion-track';
    const group=document.createElement('div');group.className='motion-group';for(const item of items)group.append(makeCard(item));
    const repeat=group.cloneNode(true);repeat.dataset.motionClone='';repeat.setAttribute('aria-hidden','true');for(const link of repeat.querySelectorAll('a,button'))link.setAttribute('tabindex','-1');track.append(group,repeat);container.append(track);
  }
  function renderActivities(root,events,helpers){
    root.replaceChildren();
    root.classList.toggle('has-event-loop',events.length>=3);
    if(!events.length){helpers.empty(root,'审核动态会随真实案件更新。每次处理都会保留时间记录。');return}
    const make=event=>{const card=document.createElement('article');card.className=events.length>=3?'activity-stream-card':'activity-item';card.append(helpers.text('strong',event.actor),helpers.text('p',event.action),helpers.text('small',helpers.date(event.date)));return card};
    if(events.length>=3)duplicateLoop(root,events,make);else for(const event of events)root.append(make(event));
    update();
  }
  const entries=[
    {title:'视频证据',copy:'保留原始录像，把时间点写清楚。',url:'/report',symbol:'▶',type:'video'},
    {title:'社区审核',copy:'按证据核实，保留独立审核意见。',url:'/communities',symbol:'◇',type:'review'},
    {title:'公开档案',copy:'查询记录，沿时间线查看处理经过。',url:'/players',symbol:'▤',type:'record'},
    {title:'申诉复核',copy:'本人 Steam 登录，提交复核材料。',url:'/account?tab=account-appeals',symbol:'↺',type:'appeal'},
  ];
  const base=document.querySelector('meta[name=site-base]')?.content||'';
  const gallery=document.getElementById('community-flow-gallery');
  if(gallery)duplicateLoop(gallery,entries,entry=>{
    const card=document.createElement('a');card.href=base+entry.url;card.className='feature-stream-card '+entry.type;
    const icon=document.createElement('span');icon.className='stream-icon';icon.setAttribute('aria-hidden','true');icon.textContent=entry.symbol;
    const heading=document.createElement('h3');heading.textContent=entry.title;
    const copy=document.createElement('p');copy.textContent=entry.copy;
    const next=document.createElement('span');next.className='stream-next';next.textContent='进入功能 ↗';card.append(icon,heading,copy,next);return card;
  });
  update();
  return {renderActivities};
})();
