const ReportTags=(()=>{
  let groups=[],labels=new Map();
  const base=document.querySelector('meta[name=site-base]')?.content||'';
  const ready=fetch(base+'/api/v1/report-categories').then(async r=>{if(!r.ok)throw Error('违规标签暂时无法加载，请刷新后再试。');groups=(await r.json()).items;labels=new Map(groups.flatMap(g=>g.items.map(t=>[t.id,t.label])));});
  function mount(root,selected=[],required=false){
    root.replaceChildren();
    const select=document.createElement('select');select.name='tags';select.required=required;select.setAttribute('aria-label','违规分类');select.setAttribute('aria-describedby',root.id==='report-tags'?'report-tag-help':'admin-tag-help');
    const placeholder=document.createElement('option');placeholder.value='';placeholder.textContent=required?'请选择一项违规分类':'未分类';select.append(placeholder);
    for(const group of groups){
      const options=document.createElement('optgroup');options.label=group.label;
      for(const tag of group.items){const option=document.createElement('option');option.value=tag.id;option.textContent=tag.label;options.append(option)}
      select.append(options);
    }
    select.value=selected[0]||'';root.append(select);
  }
  function values(form){const value=form.querySelector('select[name="tags"]')?.value;return value?[value]:[]}
  function render(tags){const root=document.createElement('div');root.className='case-tags';root.setAttribute('aria-label','举报人选择的违规标签');for(const id of tags||[]){if(!labels.has(id))continue;const chip=document.createElement('span');chip.className='case-tag';chip.textContent=labels.get(id);root.append(chip)}return root}
  return {ready,mount,values,render};
})();
