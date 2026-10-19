const PlayerProfiles=(()=>{
  const groups={all:'全部记录',active:'有效确认',expired:'历史到期',pending:'待处理',resolved:'撤销／其他'};
  const base=document.querySelector('meta[name=site-base]')?.content||'';
  let helpers,account='',group='all',page=1,size=20,version=0;
  const $=id=>document.getElementById(id);
  function steamID(raw){
    const value=String(raw||'').trim();if(/^7656119[0-9]{10}$/.test(value))return value;
    if(value.length>250)return null;
    try{const url=new URL(value);if(url.protocol!=='https:'||url.hostname!=='steamcommunity.com'||url.port||url.username||url.password)return null;return url.pathname.match(/^\/profiles\/(7656119[0-9]{10})\/?$/)?.[1]||null}catch{return null}
  }
  function route(raw){const id=steamID(raw);return id?'/search/'+id:null}
  function navigate(raw){const path=route(raw);if(path)location.href=base+path;return !!path}
  function syncURL(){const query=new URLSearchParams();if(group!=='all')query.set('group',group);if(page!==1)query.set('page',String(page));if(size!==20)query.set('size',String(size));history.replaceState(null,'',base+'/search/'+account+(query.size?'?'+query:''))}
  function identity(data){
    $('profile-title').textContent=data.displayName||'未登记昵称';$('profile-id').textContent=data.account;
    $('profile-name-source').textContent=data.displayName?'昵称来自本地公开案件；并非实时 Steam 资料。':'本地公开库尚未登记该账号昵称；可到 Steam 主页核对。';
    $('profile-last-updated').textContent=data.lastUpdated?helpers.date(data.lastUpdated):'暂无记录';$('profile-checked-at').textContent=helpers.date(data.asOf);
    $('profile-community-count').textContent=String(data.communities.length);$('profile-community-names').textContent=data.communities.length?data.communities.join('、'):'暂无关联社区';
    const official=$('profile-steam');official.href='https://steamcommunity.com/profiles/'+account;
    $('profile-report').href=base+'/report?account='+account+(data.displayName?'&player='+encodeURIComponent(data.displayName):'');
    $('profile-copy-id').disabled=false;$('profile-copy-page').disabled=false;
    const active=data.counts.active,pending=data.counts.pending,summary=$('profile-conclusion');summary.className='profile-conclusion'+(active?' has-active':pending?' has-pending':'');
    $('profile-conclusion-title').textContent=active?'有 '+active+' 条有效确认记录':pending?'有 '+pending+' 条待处理记录':'暂无有效确认记录';
    $('profile-conclusion-copy').textContent=active?'请查看具体证据、来源社区和到期时间。计数不是作弊概率。':pending?'这些记录尚无最终确认结论，不进入有效名单。':'这里只反映本站公开库；没有记录不代表账号已被证明安全。';
    const summaryRoot=$('profile-groups');summaryRoot.replaceChildren();
    for(const [key,title]of Object.entries(groups)){
      const button=helpers.text('button','','profile-group');button.type='button';button.dataset.profileGroup=key;button.setAttribute('aria-pressed',String(group===key));button.append(helpers.text('span',title),helpers.text('strong',String(key==='all'?data.publicTotal:data.counts[key])));
      button.onclick=()=>{group=key;page=1;syncURL();refresh()};summaryRoot.append(button)
    }
  }
  function expiry(rec){if(rec.group==='expired')return '已于 '+helpers.date(rec.expiresAt)+' 到期';if(rec.group==='active')return rec.expiresAt?'到期 '+helpers.date(rec.expiresAt):'未设置到期时间';if(rec.expiresAt)return '登记到期 '+helpers.date(rec.expiresAt);return rec.group==='pending'?'尚无有效确认结论':'以案件处理记录为准'}
  function recordBadge(rec){if(rec.group==='expired')return helpers.text('span','已到期','badge expired');if(rec.status==='active'&&rec.group!=='active')return helpers.text('span','到期信息待核实','badge pending');return helpers.badge(rec.status)}
  function renderRecords(data){
    $('profile-records-title').textContent=groups[group];$('profile-records-count').textContent=data.total+' 条公开案件';const root=$('profile-records');root.replaceChildren();
    if(!data.items.length){const empty=helpers.text('div','','profile-empty');empty.append(helpers.text('span','⌕','profile-empty-symbol'),helpers.text('h3',data.publicTotal?'这个分组暂无记录':'该账号暂无本站公开记录'),helpers.text('p',data.publicTotal?'可以切换其他分组，或打开具体案件查看审核过程。':'尚未登记、记录未公开或资料不足，都可能导致没有结果。请结合原始证据判断。'));root.append(empty)}
    else{
      const table=document.createElement('table');table.className='profile-record-table';const head=document.createElement('thead'),header=document.createElement('tr');for(const title of ['来源社区／游戏','原因与分类','当前状态','提交／更新','有效期限','案件']){const th=helpers.text('th',title);th.scope='col';header.append(th)}head.append(header);table.append(head);const body=document.createElement('tbody');
      for(const rec of data.items){const row=document.createElement('tr');
        const cell=(label)=>{const td=document.createElement('td');td.dataset.label=label;row.append(td);return td};
        const source=cell('来源社区／游戏');source.append(helpers.link(rec.community||'未填写社区','/players?community='+encodeURIComponent(rec.community||'')+'&q='+account),helpers.text('small',rec.game||'WARDOGS'));
        const reason=cell('原因与分类');reason.append(helpers.text('p',rec.reason||'请查看案件材料','profile-reason'),helpers.ReportTags.render(rec.tags));
        const state=cell('当前状态');state.append(recordBadge(rec));
        const dates=cell('提交／更新');dates.append(helpers.text('span','提交 '+helpers.date(rec.date)),helpers.text('small','更新 '+helpers.date(rec.updatedAt||rec.date)));
        cell('有效期限').append(helpers.text('span',expiry(rec)));
        const action=cell('案件');action.append(helpers.link('查看案件','/player/'+encodeURIComponent(rec.id),'profile-case-link'),helpers.text('small',rec.id));
        body.append(row)
      }table.append(body);root.append(table)
    }
    const pages=Math.max(1,Math.ceil(data.total/size));$('profile-page-info').textContent=page+' / '+pages+' 页';$('profile-previous').disabled=page<=1;$('profile-next').disabled=page>=pages;
  }
  async function refresh(){
    const request=++version;$('profile-records').setAttribute('aria-busy','true');$('profile-records-feedback').textContent='正在读取公开记录…';
    try{const data=await helpers.api('profiles/'+account+'?'+new URLSearchParams({group,page:String(page),size:String(size)}));if(request!==version)return;identity(data);renderRecords(data);$('profile-records-feedback').textContent=''}
    catch(error){if(request===version){$('profile-records-feedback').textContent=error.message;$('profile-records').replaceChildren(helpers.text('p','未能完成查询，请稍后重试。'));$('profile-previous').disabled=true;$('profile-next').disabled=true}}
    finally{if(request===version)$('profile-records').removeAttribute('aria-busy')}
  }
  async function load(id,tools){
    helpers=tools;$('profile-search-form').onsubmit=event=>{event.preventDefault();const input=$('profile-query');if(!navigate(input.value)){input.setCustomValidity('请填写17位 SteamID64，或 https://steamcommunity.com/profiles/数字ID 的链接。');input.reportValidity()}};
    $('profile-query').oninput=()=>{$('profile-query').setCustomValidity('')};
    const copy=async value=>{try{await navigator.clipboard.writeText(value);$('profile-action-feedback').textContent='已复制。'}catch{$('profile-action-feedback').textContent='无法自动复制，请手动复制账号或地址栏。'}};
    $('profile-copy-id').onclick=()=>copy(account);$('profile-copy-page').onclick=()=>copy(location.origin+base+'/search/'+account);
    $('profile-previous').onclick=()=>{page--;syncURL();refresh()};$('profile-next').onclick=()=>{page++;syncURL();refresh()};$('profile-page-size').onchange=event=>{size=Number(event.target.value);page=1;syncURL();refresh()};$('profile-refresh').onclick=()=>refresh();
    if(!/^7656119[0-9]{10}$/.test(id)){$('profile-title').textContent='账号地址无效';$('profile-records-feedback').textContent='请在上方重新输入有效 SteamID64。';return}
    account=id;$('profile-query').value=id;const query=new URLSearchParams(location.search);if(Object.hasOwn(groups,query.get('group')))group=query.get('group');const queryPage=Number(query.get('page'));if(Number.isInteger(queryPage)&&queryPage>=1&&queryPage<=1000000)page=queryPage;if(query.get('size')==='50')size=50;$('profile-page-size').value=String(size);document.title='账号 '+id+' · 社区联合黑名单';await refresh();
  }
  return {steamID,route,navigate,load};
})();
