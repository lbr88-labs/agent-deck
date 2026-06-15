import{render as bn,html as yn}from"htm/preact";import{html as $n}from"htm/preact";import{useEffect as Fs}from"preact/hooks";import{html as R}from"htm/preact";import{useEffect as Ae}from"preact/hooks";import{html as pe}from"htm/preact";import{html as pt}from"htm/preact";function ge(){return pt`
    <svg width="28" height="18" viewBox="0 0 120 80" aria-hidden="true">
      <rect fill="#1a1b26" width="120" height="80" rx="12" stroke="var(--border-hi)" stroke-width="1"/>
      <line x1="40" y1="8" x2="40" y2="72" stroke="#414868" stroke-width="1.5"/>
      <line x1="80" y1="8" x2="80" y2="72" stroke="#414868" stroke-width="1.5"/>
      <circle cx="20" cy="40" r="11" fill="var(--tn-green)"/>
      <circle cx="60" cy="40" r="11" fill="var(--tn-yellow)"/>
      <circle cx="100" cy="40" r="11" fill="var(--tn-muted-2)"/>
    </svg>
  `}function k({d:e,size:s=14}){return pt`
    <svg width=${s} height=${s} viewBox="0 0 24 24" fill="none" stroke="currentColor"
         stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
      <path d=${e}/>
    </svg>
  `}var x={play:"M6 4l14 8-14 8V4z",stop:"M6 6h12v12H6z",restart:"M4 4v5h5 M20 20v-5h-5 M20 9a8 8 0 00-14-3 M4 15a8 8 0 0014 3",fork:"M6 3v6a3 3 0 003 3h6a3 3 0 013 3v3 M6 3v0 M6 21v-6 M18 21v0 M6 21v0 M6 9v0",trash:"M3 6h18 M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2 M6 6v14a2 2 0 002 2h8a2 2 0 002-2V6",plus:"M12 5v14 M5 12h14",filter:"M3 5h18 M6 12h12 M10 19h4",search:"M11 2a9 9 0 100 18 9 9 0 000-18z M22 22l-5-5",settings:"M12 8a4 4 0 100 8 4 4 0 000-8z M12 2v2 M12 20v2 M4.93 4.93l1.41 1.41 M17.66 17.66l1.41 1.41 M2 12h2 M20 12h2 M4.93 19.07l1.41-1.41 M17.66 6.34l1.41-1.41",chev:"M6 9l6 6 6-6",chevR:"M9 6l6 6-6 6",x:"M6 6l12 12 M6 18L18 6",zap:"M13 2L3 14h8l-1 8 10-12h-8l1-8z",wifi:"M5 12a10 10 0 0114 0 M8.5 15.5a5 5 0 017 0 M12 19h.01",send:"M22 2L11 13 M22 2l-7 20-4-9-9-4 20-7z",book:"M4 4h12a4 4 0 014 4v12H8a4 4 0 01-4-4z M4 4v16",term:"M4 4h16v16H4z M8 9l3 3-3 3 M13 15h4",edit:"M12 20h9 M16.5 3.5a2.121 2.121 0 013 3L7 19l-4 1 1-4 12.5-12.5z"};function Ge({status:e,size:s=7}){return pt`<span class=${`dot ${e||"idle"}`} style=${{width:s+"px",height:s+"px"}}/>`}function At(e){return e==="conductor"?"\u25C6":e==="watcher"?"\u25C7":"\u203A"}import{computed as Zs}from"@preact/signals";import{signal as y}from"@preact/signals";import{html as he}from"htm/preact";var Ks=0,Bs=50,qs=5e3,Js="agentdeck_toast_history";function vt(e){if(!e)return;let s=[...$e.value,e].slice(-Bs);$e.value=s;try{localStorage.setItem(Js,JSON.stringify(s))}catch{}}function A(e,s){let t=s||"error",a={id:++Ks,message:e,type:t,createdAt:Date.now()},i=[...ce.value,a];if(i.length>3){let n=i.findIndex(o=>o.type!=="error");if(n>=0){let[o]=i.splice(n,1);vt(o)}else{let o=i.shift();vt(o)}}ce.value=i,a.type!=="error"&&setTimeout(()=>Pt(a.id),qs)}function Pt(e){let s=ce.value.find(t=>t.id===e);s&&vt(s),ce.value=ce.value.filter(t=>t.id!==e)}function Ot({id:e,message:s,type:t}){let a=t==="error"?"var(--tn-red)":t==="info"?"var(--accent)":"var(--tn-green)";return he`
    <div class="toast" data-testid="toast" style=${{borderColor:a,position:"relative",pointerEvents:"auto"}}>
      <span class="t" style=${{color:a}}>${t==="error"?"\u2715":t==="info"?"\u2139":"\u2713"}</span>
      <span style="margin-left: 6px;">${s}</span>
      <button type="button"
        onClick=${()=>Pt(e)}
        aria-label="Dismiss"
        data-testid="toast-dismiss"
        style="background: transparent; border: 0; color: var(--muted); cursor: pointer;
               margin-left: 10px; padding: 0 4px; font-size: 12px;">✕</button>
    </div>
  `}function Mt(){let e=ce.value;if(e.length===0)return null;let s=e.filter(a=>a.type==="error"),t=e.filter(a=>a.type!=="error");return he`
    <div style=${{position:"fixed",bottom:"40px",right:"14px",zIndex:70,display:"flex",flexDirection:"column",gap:"6px",pointerEvents:"none",maxWidth:"420px"}}>
      ${s.length>0&&he`
        <div role="alert" aria-live="assertive" style=${{display:"flex",flexDirection:"column",gap:"6px"}}>
          ${s.map(a=>he`<${Ot} key=${a.id} ...${a}/>`)}
        </div>
      `}
      ${t.length>0&&he`
        <div role="status" aria-live="polite" style=${{display:"flex",flexDirection:"column",gap:"6px"}}>
          ${t.map(a=>he`<${Ot} key=${a.id} ...${a}/>`)}
        </div>
      `}
    </div>
  `}async function b(e,s,t){let a={"Content-Type":"application/json",Accept:"application/json"},i=se.value;i&&(a.Authorization="Bearer "+i);let n;try{n=await fetch(s,{method:e,headers:a,body:t?JSON.stringify(t):void 0})}catch(l){let d="Network error: "+(l.message||"request failed");throw A(d),new Error(d)}let o=await n.json();if(!n.ok){let l=o?.error?.message||n.statusText;throw e!=="GET"&&A(l),new Error(l)}return o}var de=y([]),je=y([]),w=y(null),Y=y("connecting"),Ln=y(localStorage.getItem("theme")||"system"),mt=y(null),se=y(""),Lt=y({});function Vs(){try{let e=localStorage.getItem("agentdeck.sidebarOpen");if(e==="true")return!0;if(e==="false")return!1}catch{}return typeof window<"u"&&window.innerWidth>=768}var Rn=y(Vs()),Dt=200,Nt=480,Rt=280;function Ys(e){return Number.isFinite(e)?e<Dt?Dt:e>Nt?Nt:Math.round(e):Rt}function Xs(){try{let e=localStorage.getItem("sidebar-width");if(e!=null){let s=parseInt(e,10);return Ys(s)}}catch{}return Rt}var Fn=y(Xs());var zn=y(null),G=y(!1),W=y(null),be=y(null),Pe=y(null),Z=y("disconnected"),We=y(!1),_n=y(null),Un=y(!1),Gn=y(!1),jn=y(""),ne=y(!1),Wn=y(""),Hn=y(!1),ce=y([]),ee=y(!1);function Qs(){try{let e=localStorage.getItem("agentdeck_toast_history");if(e){let s=JSON.parse(e);if(Array.isArray(s))return s.slice(-50)}}catch{}return[]}var $e=y(Qs()),ue=y(!1),M=y(!0),Ft=y(!1),zt=y([]),He=y(!1),_t=y([]),ye=y([]),ft=y(!1),Ke=y(null),Be=y(null),te=y(null),ke=y(null),xe=y(null),ae=y([]);function Ut(e){let s=[e,...ae.value].slice(0,8);ae.value=s}function Se(e,s){ae.value=ae.value.map(t=>t.correlationId===e?{...t,...s}:t)}async function qe(){try{let e=await b("GET","/api/sessions/archived");je.value=e.sessions||[]}catch{je.value=[]}}function ea(e){return!e||!e.tool?"agent":e.groupPath==="conductor"||/conductor/i.test(e.title||"")?"conductor":["webhook","ntfy","slack-watcher"].includes(e.tool)?"watcher":"agent"}function ta(e){let s=e.session||{},t=s.id||"",a=s.groupPath||"";return{id:t,kind:ea(s),title:s.title||t,group:a,tool:s.tool||"",modelId:s.modelId||"",model:s.model||"",modelVersion:s.modelVersion||"",canFork:!!s.canFork,status:s.status||"idle",branch:s.branch||"\u2014",path:s.projectPath||"",cost:0,tokens:0,mcps:[],skills:[],children:[],worktree:!!(s.worktreeBranch&&s.worktreeRepoRoot),worktreeBranch:s.worktreeBranch||"",sandbox:!1,parent:null,pendingNeeds:0,watcherType:null,routes:"",events1h:0,meta:"",raw:s}}function sa(e){let s=e.group||{};return{path:s.path||"",label:(s.name||s.path||"").toUpperCase(),expanded:!!s.expanded,sessionCount:s.sessionCount||0,order:s.order||0,kind:s.path==="conductor"?"conductor":s.path==="watchers"?"watcher":null}}var L=Zs(()=>{let e=de.value||[],s=Lt.value||{},t=[],a=[];for(let o of e)if(o){if(o.type==="group")t.push(sa(o));else if(o.type==="session"){let l=ta(o),d=s[l.id];typeof d=="number"&&(l.cost=d),a.push(l)}}let i=new Set(t.map(o=>o.path));for(let o of a)o.group&&!i.has(o.group)&&(t.push({path:o.group,label:o.group.toUpperCase(),expanded:!0,sessionCount:0,order:999,kind:null}),i.add(o.group));t.sort((o,l)=>o.order-l.order);let n={};for(let o of a)(n[o.group]||=[]).push(o);return{groups:t,sessions:a,byGroup:n}});import{signal as V,effect as Gt}from"@preact/signals";function Te(e,s){try{let t=localStorage.getItem(e);return t==null?s:JSON.parse(t)}catch{return s}}function Ee(e,s){Gt(()=>{try{localStorage.setItem(s,JSON.stringify(e.value))}catch{}})}var D=V(Te("agentdeck.tab","fleet"));Ee(D,"agentdeck.tab");var oe=V(!1),X=V(!1),we=V(Te("agentdeck.accent","blue"));Ee(we,"agentdeck.accent");var Ce=V(Te("agentdeck.density","balanced"));Ee(Ce,"agentdeck.density");var H=V(Te("agentdeck.rail","visible"));Ee(H,"agentdeck.rail");var Je=V(Te("agentdeck.rightRailPanels",{overview:!0,usage:!0,mcps:!0,skills:!0,children:!0,events:!0}));Ee(Je,"agentdeck.rightRailPanels");var Ve=V([]),Yn=V("fleet"),Ye=V(Te("agentdeck.showCols",{tool:!0,cost:!0,branch:!1,attach:!1,sandbox:!1,lastSeen:!1}));Ee(Ye,"agentdeck.showCols");var ie=V("");Gt(()=>{typeof document>"u"||(document.documentElement.dataset.accent=we.value,document.documentElement.dataset.density=Ce.value,document.documentElement.dataset.rail=H.value,document.body.dataset.accent=we.value,document.body.dataset.density=Ce.value,document.body.dataset.rail=H.value)});import{html as Me}from"htm/preact";function aa(e){if(!e)return"";try{return new Date(e).toLocaleTimeString([],{hour:"2-digit",minute:"2-digit",second:"2-digit"})}catch{return""}}function jt(){let e=$e.value.length;return Me`
    <button type="button"
      class=${`icon-btn ${ue.value?"active":""}`}
      onClick=${()=>{ue.value=!ue.value}}
      aria-label=${"Toast history ("+e+" entries)"}
      aria-expanded=${ue.value}
      title="Toast history"
      data-testid="toast-history-toggle"
      style="position: relative;">
      <svg width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"
           stroke-linecap="round" stroke-linejoin="round" viewBox="0 0 24 24">
        <path d="M12 8v4l3 3"/>
        <circle cx="12" cy="12" r="9"/>
      </svg>
      ${e>0&&Me`<span class="pip" style="background: var(--accent); box-shadow: 0 0 6px var(--accent);"/>`}
    </button>
  `}function Wt(){if(!ue.value)return null;let e=$e.value,s=()=>{ue.value=!1};return Me`
    <div class="overlay" role="dialog" aria-modal="true" aria-label="Toast history"
         data-testid="toast-history-drawer"
         style="justify-content: flex-end; padding: 0;"
         onClick=${t=>{t.target===t.currentTarget&&s()}}>
      <div class="dialog" style="width: 420px; max-width: 100vw; height: 100vh; max-height: 100vh; border-radius: 0; border-right: 0;"
           onClick=${t=>t.stopPropagation()}>
        <div class="dh">
          <span class="kicker">HISTORY</span>
          <div class="t">Toast history</div>
          <button type="button" class="icon-btn" onClick=${s} aria-label="Close toast history">
            <${k} d=${x.x}/>
          </button>
        </div>
        <div class="db" style="padding: 0;">
          ${e.length===0&&Me`
            <div style="padding: 20px; font-family: var(--mono); font-size: 12px; color: var(--muted); text-align: center;">
              No dismissed toasts yet.
            </div>
          `}
          ${e.slice().reverse().map(t=>Me`
            <div key=${t.id}
                 data-testid="toast-history-entry"
                 style=${{padding:"10px 14px",borderBottom:"1px solid var(--border)",background:t.type==="error"?"rgba(247,118,142,0.06)":"transparent"}}>
              <div style=${{fontFamily:"var(--mono)",fontSize:"10px",color:t.type==="error"?"var(--tn-red)":"var(--muted)",letterSpacing:"0.06em",marginBottom:"4px"}}>
                ${aa(t.createdAt)} · ${t.type}
              </div>
              <div style=${{fontSize:"12.5px",color:t.type==="error"?"var(--tn-red)":"var(--text)"}}>${t.message}</div>
            </div>
          `)}
        </div>
      </div>
    </div>
  `}var na=[{id:"command-center",label:"Command Center"},{id:"fleet",label:"Fleet"},{id:"terminal",label:"Terminal"},{id:"mcp",label:"MCPs"},{id:"skills",label:"Skills"},{id:"conductor",label:"Conductor"},{id:"watchers",label:"Watchers"},{id:"costs",label:"Costs"},{id:"search",label:"Search"},{id:"archived",label:"Archived"}];function Ht(){let e=D.value,s=Y.value,t=H.value,{sessions:a}=L.value,i=a.filter(c=>c.status==="waiting"||c.status==="error").length,n=a.reduce((c,h)=>c+(h.pendingNeeds||0),0),o=te.value,l=o&&Array.isArray(o.decisionsWaiting)?o.decisionsWaiting.length:0,d=s==="connected"?"":"off",r=s==="connected"?{}:{background:"var(--tn-red)",boxShadow:"0 0 6px var(--tn-red)"};return pe`
    <header class="topbar">
      <div class="top-brand">
        <${ge}/>
        <div class="brand-text">agent-deck<span class="dim">web</span></div>
      </div>
      <div class="top-mid">
        <button class="top-search" onClick=${()=>oe.value=!0}>
          <${k} d=${x.search} size=${13}/>
          <input readonly placeholder="Jump to session, search conversations, run command…"/>
          <span class="kbd">⌘K</span>
        </button>
        <div class="top-tabs">
          ${na.map(c=>pe`
            <button key=${c.id}
              class=${`top-tab ${e===c.id?"active":""}`}
              onClick=${()=>D.value=c.id}>
              ${c.label}
              ${c.id==="conductor"&&n>0&&pe`<span class="badge">${n}</span>`}
              ${c.id==="fleet"&&i>0&&pe`<span class="badge">${i}</span>`}
              ${c.id==="command-center"&&l>0&&pe`<span class="badge">${l}</span>`}
            </button>
          `)}
        </div>
      </div>
      <div class="top-right">
        <div class=${`conn-pill ${d}`}>
          <span class="dot" style=${r}/>ws · ${s==="connected"?"live":s}
        </div>
        ${(()=>{let c=Ke.value,h=c&&Array.isArray(c.profiles)?c.profiles:null;if(!h||h.length===0)return null;let m=ie.value||c.current||h[0];return pe`
            <span class="icon-btn"
              style=${{width:"auto",padding:"0 8px",fontFamily:"var(--mono)",fontSize:"11px",cursor:"default"}}
              title="Active profile (bound at startup; not switchable from the web UI)">
              ${m}
            </span>
          `})()}
        <${jt}/>
        <button
          class=${`icon-btn ${t==="visible"?"active":""}`}
          onClick=${()=>H.value=t==="visible"?"hidden":"visible"}
          title=${t==="visible"?"Hide right rail (])":"Show right rail (])"}
          aria-label="Toggle right rail">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor"
               stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="3" width="18" height="18" rx="2"/>
            <line x1="15" y1="3" x2="15" y2="21"/>
            ${t==="visible"&&pe`<line x1="18" y1="8" x2="18" y2="16" opacity="0.5"/>`}
          </svg>
        </button>
        <button class="icon-btn" onClick=${()=>X.value=!X.value} title="Tweaks" aria-label="Tweaks">
          <${k} d=${x.settings}/>
        </button>
      </div>
    </header>
  `}import{html as N}from"htm/preact";import{useState as Xe,useMemo as oa}from"preact/hooks";var ia=[{id:"running",sym:"\u25CF"},{id:"waiting",sym:"\u25D0"},{id:"error",sym:"\u2715"},{id:"idle",sym:"\u25CB"}],la=[{id:"tool",label:"Tool badge"},{id:"cost",label:"Cost"},{id:"branch",label:"Git branch"},{id:"attach",label:"MCPs / skills"},{id:"sandbox",label:"Docker / worktree"},{id:"lastSeen",label:"Last activity"}];function le(e,s){if(!M.value){A("mutations disabled");return}let t=s.id;if(e==="start")return b("POST",`/api/sessions/${t}/start`).catch(()=>{});if(e==="stop")return b("POST",`/api/sessions/${t}/stop`).catch(()=>{});if(e==="restart")return b("POST",`/api/sessions/${t}/restart`).catch(()=>{});if(e==="fork")return b("POST",`/api/sessions/${t}/fork`,{title:s.title+"-fork"}).catch(()=>{});if(e==="archive"&&(W.value={message:`Archive session "${s.title}"? The process will be stopped and hidden from the active list.`,onConfirm:()=>b("POST",`/api/sessions/${t}/archive`).then(()=>{w.value===t&&(w.value=null,window.location.pathname.startsWith("/s/")&&history.replaceState(null,"","/"))}).catch(()=>{})}),e==="delete"&&(W.value={message:`Delete session "${s.title}"? This stops the tmux session and removes metadata.`,onConfirm:()=>b("DELETE",`/api/sessions/${t}`).catch(()=>{})}),e==="worktreeFinish"){let a=s.worktreeBranch||s.branch;W.value={message:`Finish worktree for "${s.title}"? Merges branch "${a}" into default branch, removes worktree, deletes branch, and removes session.`,onConfirm:()=>b("POST",`/api/sessions/${t}/worktree/finish`).catch(()=>{})}}e==="edit"&&(Pe.value={sessionId:t})}function ra({s:e,sel:s,onSelect:t,showCols:a}){let[i,n]=Xe(!1),o=(e.mcps||[]).length,l=(e.skills||[]).length,d=a.branch&&e.branch&&e.branch!=="\u2014"||a.attach&&(o>0||l>0)||a.sandbox&&(e.sandbox||e.worktree)||a.lastSeen;return N`
    <div class=${`sess ${s?"sel":""} ${e.kind} ${i?"exp":""}`} onClick=${()=>t(e.id)}>
      <span class="sig">${At(e.kind)}</span>
      <div class="titleline">
        <${Ge} status=${e.status}/>
        <span class="tt">${e.title}</span>
      </div>
      <div class="meta">
        ${a.tool&&e.tool&&N`<span class="tag">${e.tool}</span>`}
        ${a.cost&&e.cost>0&&N`<span class="cost">$${e.cost.toFixed(2)}</span>`}
        <button class="row-chev" title="Details" onClick=${r=>{r.stopPropagation(),n(c=>!c)}}>
          ${i?"\u25BE":"\u25B8"}
        </button>
      </div>
      ${d&&N`
        <div class="subline">
          ${a.branch&&e.branch&&e.branch!=="\u2014"&&N`<span class="trunc"><span class="b">git</span> ${e.branch}</span>`}
          ${a.attach&&o>0&&N`<span class="att-count">${o} mcp${o>1?"s":""}</span>`}
          ${a.attach&&l>0&&N`<span class="att-count skill">${l} skill${l>1?"s":""}</span>`}
          ${a.sandbox&&e.sandbox&&N`<span class="att-count warn">docker</span>`}
          ${a.sandbox&&e.worktree&&N`<span class="att-count">worktree</span>`}
        </div>
      `}
      ${i&&N`
        <div class="row-detail" onClick=${r=>r.stopPropagation()}>
          <div class="rd-row"><span class="rd-k">tool</span><span class="rd-v">${e.tool||"\u2014"}</span></div>
          ${e.branch&&e.branch!=="\u2014"&&N`<div class="rd-row"><span class="rd-k">branch</span><span class="rd-v">${e.branch}</span></div>`}
          ${e.path&&N`<div class="rd-row"><span class="rd-k">path</span><span class="rd-v" title=${e.path}>${e.path}</span></div>`}
          ${e.cost>0&&N`<div class="rd-row"><span class="rd-k">cost</span><span class="rd-v ok">$${e.cost.toFixed(2)}</span></div>`}
        </div>
      `}
      <div class="actions" onClick=${r=>r.stopPropagation()}>
        ${e.status==="running"||e.status==="waiting"?N`<button class="mini" title="Stop" data-testid="session-stop-btn" onClick=${()=>le("stop",e)}><${k} d=${x.stop} size=${12}/></button>`:N`<button class="mini good" title="Start" data-testid="session-start-btn" onClick=${()=>le("start",e)}><${k} d=${x.play} size=${12}/></button>`}
        <button class="mini good" title="Restart" data-testid="session-restart-btn" onClick=${()=>le("restart",e)}><${k} d=${x.restart} size=${12}/></button>
        <button class="mini" title="Edit" data-testid="edit-session-btn" onClick=${()=>le("edit",e)}><${k} d=${x.edit} size=${12}/></button>
        ${e.canFork&&N`<button class="mini fork" title="Fork" data-testid="session-fork-btn" onClick=${()=>le("fork",e)}><${k} d=${x.fork} size=${12}/></button>`}
        ${e.worktree&&N`<button class="mini" title="Finish worktree (merge + cleanup)" onClick=${()=>le("worktreeFinish",e)} data-action="worktree-finish" data-testid="session-worktree-finish-btn">⎇✓</button>`}
        <button class="mini" title="Archive" onClick=${()=>le("archive",e)}>⌂</button>
        <button class="mini danger" title="Delete" data-testid="session-delete-btn" onClick=${()=>le("delete",e)}><${k} d=${x.trash} size=${12}/></button>
      </div>
    </div>
  `}function Kt(){let{groups:e,byGroup:s,sessions:t}=L.value,a=w.value,i=Ve.value,n=Ye.value,[o,l]=Xe(""),[d,r]=Xe(!1),[c,h]=Xe(()=>Object.fromEntries(e.map(u=>[u.path,u.expanded!==!1]))),m=u=>{if(i.length&&!i.includes(u.status))return!1;if(!o)return!0;let T=o.toLowerCase();return((u.title||"")+" "+(u.group||"")+" "+(u.path||"")+" "+(u.tool||"")+" "+(u.branch||"")).toLowerCase().includes(T)},S=oa(()=>t.filter(m).length,[t,o,i]),C=u=>{let T=Ve.value;Ve.value=T.includes(u)?T.filter($=>$!==u):[...T,u]},f=u=>h(T=>({...T,[u]:T[u]===!1})),I=u=>{w.value=u,D.value="terminal"},v=u=>{Ye.value={...n,[u]:!n[u]}};return N`
    <div class="sidebar">
      <div class="side-head">
        <span class="label">SESSIONS</span>
        <span class="count">${S}</span>
        <div class="spacer"/>
        <div style="position: relative;">
          <button class=${`icon-btn ${d?"active":""}`} title="Show columns" aria-label="Show columns"
                  data-testid="show-cols-btn"
                  onClick=${()=>r(u=>!u)}>
            <${k} d=${x.filter}/>
          </button>
          ${d&&N`
            <div class="show-menu" data-testid="show-cols-menu" onClick=${u=>u.stopPropagation()}>
              <div class="sm-head">SHOW IN ROW</div>
              ${la.map(u=>N`
                <label key=${u.id} class="sm-row" data-testid=${`show-col-${u.id}`}>
                  <input type="checkbox" checked=${!!n[u.id]} onChange=${()=>v(u.id)}/>
                  <span>${u.label}</span>
                </label>
              `)}
              <div class="sm-foot" onClick=${()=>r(!1)}>done</div>
            </div>
          `}
        </div>
        ${M.value&&N`
          <button class="icon-btn" title="New session (n)" aria-label="New session"
                  onClick=${()=>G.value=!0}>
            <${k} d=${x.plus}/>
          </button>
        `}
      </div>
      <div class="side-filter">
        <input
          placeholder="/ filter"
          data-testid="sidebar-filter-input"
          value=${o}
          onInput=${u=>l(u.target.value)}
        />
        ${ia.map(u=>N`
          <span key=${u.id}
                class=${`side-chip ${i.includes(u.id)?"on":""}`}
                data-testid=${`status-chip-${u.id}`}
                onClick=${()=>C(u.id)}
                title=${u.id}>
            ${u.sym}
          </span>
        `)}
      </div>
      <div class="side-list">
        ${e.map(u=>{let T=(s[u.path]||[]).filter(m);if(o&&T.length===0)return null;let $=c[u.path]!==!1;return N`
            <div key=${u.path}>
              <div class=${`side-group-head ${u.kind||""}`} data-testid=${`group-head-${u.path}`} onClick=${()=>f(u.path)}>
                <span class="chev">${$?"\u25BE":"\u25B8"}</span>
                <span class="name">${u.label}</span>
                <span class="badge">(${T.length})</span>
              </div>
              ${$&&T.map(E=>N`
                <${ra} key=${E.id} s=${E} sel=${a===E.id} onSelect=${I} showCols=${n}/>
              `)}
            </div>
          `})}
        ${t.length===0&&N`
          <div style="padding: 16px; font-family: var(--mono); font-size: 11px; color: var(--muted); text-align: center;">
            No sessions yet. Press <span class="kbd" style="border:1px solid var(--border); padding: 0 4px; border-radius: 3px;">n</span> to create one.
          </div>
        `}
      </div>
    </div>
  `}import{html as Qe}from"htm/preact";function Bt(){let{sessions:e}=L.value,s=Y.value,t=e.filter(r=>r.status==="running").length,a=e.filter(r=>r.status==="waiting").length,i=e.filter(r=>r.status==="error").length,n=s==="connected"?{}:{background:"var(--tn-red)",boxShadow:"0 0 6px var(--tn-red)"},o=Be.value,l=o?.cpu?.usage_percent,d=o?.memory?.usage_percent;return Qe`
    <div class="footer">
      <span class="fseg"><span class="d" style=${n}/>ws · ${s}</span>
      ${ie.value&&Qe`<span class="fseg">profile · ${ie.value}</span>`}
      <span class="fseg">sessions · ${e.length}</span>
      <span class="fseg" style="color: var(--tn-green);">● ${t}</span>
      <span class="fseg" style="color: var(--tn-yellow);">◐ ${a}</span>
      <span class="fseg" style="color: var(--tn-red);">✕ ${i}</span>
      ${typeof l=="number"&&Qe`<span class="fseg">cpu · ${l.toFixed(0)}%</span>`}
      ${typeof d=="number"&&Qe`<span class="fseg">mem · ${d.toFixed(0)}%</span>`}
      <span class="fspacer"/>
      <span class="fkbd"><span class="k">⌘K</span> palette</span>
      <span class="fkbd"><span class="k">/</span> filter</span>
      <span class="fkbd"><span class="k">n</span> new</span>
      <span class="fkbd"><span class="k">?</span> tweaks</span>
      <span class="fkbd"><span class="k">]</span> rail</span>
    </div>
  `}import{html as P}from"htm/preact";import{signal as ca}from"@preact/signals";var qt=ca({}),da=[{id:"overview",label:"Overview"},{id:"usage",label:"Usage & activity"},{id:"mcps",label:"MCPs"},{id:"skills",label:"Skills"},{id:"children",label:"Children (conductor)"},{id:"events",label:"Events (watcher)"}];function K({title:e,badge:s,testid:t,children:a}){return P`
    <div class="card" data-testid=${t}>
      <div class="card-head">
        <span class="name">${e}</span>
        ${s&&P`<span class="pill">${s}</span>`}
      </div>
      <div class="card-body">${a}</div>
    </div>
  `}function De({msg:e}){return P`<div style="font-family: var(--mono); font-size: 11px; color: var(--muted);">${e}</div>`}function ua(e,s){let t=new Map;for(let n of s){let o=n.raw&&n.raw.parentSessionId;o&&(t.has(o)||t.set(o,[]),t.get(o).push(n))}let a=new Set([e]),i=n=>(t.get(n)||[]).filter(l=>a.has(l.id)?!1:(a.add(l.id),!0)).map(l=>({session:l,children:i(l.id)}));return i(e)}function Jt({node:e,depth:s,rootId:t}){let a=qt.value,i=t+":"+e.session.id,n=!a[i],o=e.children.length>0,l=()=>{qt.value={...a,[i]:n}};return P`
    <div class="child-node" data-session-id=${e.session.id} data-depth=${s}
         style="font-family: var(--mono); font-size: 11px; line-height: 1.7; padding-left: ${s*12}px;">
      <span class="child-row" style="display: inline-flex; align-items: center; gap: 4px;">
        <span class="child-toggle"
              onClick=${o?l:null}
              style=${`width: 10px; display: inline-block; cursor: ${o?"pointer":"default"}; color: var(--muted);`}>
          ${o?n?"\u25BE":"\u25B8":" "}
        </span>
        <span class="child-status pill" data-status=${e.session.status}
              style="font-size: 9px; padding: 0 4px;">${e.session.status}</span>
        <span class="child-title" style="color: var(--text-hi);">${e.session.title}</span>
        ${e.session.tool&&P`<span class="child-tool" style="color: var(--muted);">· ${e.session.tool}</span>`}
      </span>
      ${o&&n&&e.children.map(d=>P`
        <${Jt} key=${d.session.id} node=${d} depth=${s+1} rootId=${t}/>
      `)}
    </div>
  `}function pa({rootId:e,sessions:s}){let t=ua(e,s);return t.length===0?P`<${De} msg="No child sessions yet."/>`:P`
    <div class="children-tree" data-children-count=${t.length}>
      ${t.map(a=>P`
        <${Jt} key=${a.session.id} node=${a} depth=${0} rootId=${e}/>
      `)}
    </div>
  `}function Vt(){let{sessions:e}=L.value,s=w.value,t=e.find(n=>n.id===s)||e[0],a=Je.value;if(!t)return P`
      <div class="rightrail" data-testid="right-rail">
        <div class="rail-head"><span class="t">SESSION</span></div>
        <div class="rail-body">
          <div style="padding: 18px; font-family: var(--mono); font-size: 11px; color: var(--muted);">
            no session selected
          </div>
        </div>
      </div>
    `;let i=n=>{Je.value={...a,[n]:!a[n]}};return P`
    <div class="rightrail" data-testid="right-rail">
      <div class="rail-head">
        <span class="t">SESSION</span>
        <div class="spacer"/>
        <span class="t" style="color: var(--text-hi);">${t.title}</span>
      </div>
      <div class="rail-body">
        ${a.overview&&P`
          <${K} title="OVERVIEW" badge=${t.status} testid="rail-card-overview">
            <div class="kv"><span class="k">kind</span><span class="v">${t.kind}</span></div>
            <div class="kv"><span class="k">tool</span><span class="v">${t.tool||"\u2014"}</span></div>
            ${t.model&&P`
              <div class="kv"><span class="k">model</span><span class="v">${t.model}</span></div>`}
            ${t.modelVersion&&P`
              <div class="kv"><span class="k">version</span><span class="v">${t.modelVersion}</span></div>`}
            ${t.modelId&&P`
              <div class="kv"><span class="k">model id</span><span class="v" title=${t.modelId}>${t.modelId}</span></div>`}
            <div class="kv"><span class="k">group</span><span class="v">${t.group||"\u2014"}</span></div>
            ${t.branch&&t.branch!=="\u2014"&&P`
              <div class="kv"><span class="k">branch</span><span class="v">${t.branch}</span></div>`}
            ${t.path&&P`
              <div class="kv"><span class="k">path</span><span class="v" title=${t.path}>${t.path}</span></div>`}
            ${t.sandbox&&P`<div class="kv"><span class="k">sandbox</span><span class="v warn">docker</span></div>`}
            ${t.worktree&&P`<div class="kv"><span class="k">worktree</span><span class="v ok">yes</span></div>`}
          </${K}>
        `}
        ${a.usage&&P`
          <${K} title="USAGE" testid="rail-card-usage">
            ${t.cost>0?P`<div class="kv"><span class="k">cost</span><span class="v ok">$${t.cost.toFixed(2)}</span></div>`:P`<${De} msg="cost data not available for this session"/>`}
            ${t.tokens>0&&P`<div class="kv"><span class="k">tokens</span><span class="v">${(t.tokens/1e3).toFixed(1)}k</span></div>`}
          </${K}>
        `}
        ${a.mcps&&P`
          <${K} title="MCPS" testid="rail-card-mcps">
            <${De} msg="MCP attachments not exposed via web API. Use TUI (m key)."/>
          </${K}>
        `}
        ${a.skills&&P`
          <${K} title="SKILLS" testid="rail-card-skills">
            <${De} msg="Skill attachments not exposed via web API. Use TUI (s key)."/>
          </${K}>
        `}
        ${a.children&&t.kind==="conductor"&&P`
          <${K} title="CHILDREN" badge="conductor" testid="rail-card-children">
            <${pa} rootId=${t.id} sessions=${e}/>
          </${K}>
        `}
        ${a.events&&t.kind==="watcher"&&P`
          <${K} title="EVENTS" testid="rail-card-events">
            <${De} msg="Watcher event stream not exposed via web API."/>
          </${K}>
        `}
        <div class="rail-add">
          <div>Right-rail panels</div>
          <div class="opts">
            ${da.map(n=>P`
              <span key=${n.id}
                    data-testid=${`rail-panel-toggle-${n.id}`}
                    class=${`opt ${a[n.id]?"on":""}`}
                    onClick=${()=>i(n.id)}>
                ${n.label}
              </span>
            `)}
          </div>
        </div>
      </div>
    </div>
  `}import{html as Yt}from"htm/preact";var va=[{id:"command-center",label:"Command",icon:"\u2605"},{id:"fleet",label:"Fleet",icon:"\u25A6"},{id:"terminal",label:"Session",icon:"\u203A_"},{id:"watchers",label:"Watchers",icon:"\u25C7"},{id:"costs",label:"Costs",icon:"$"}];function Xt(){let e=D.value;return Yt`
    <div class="mob-tabs" data-testid="mobile-tabs">
      ${va.map(s=>Yt`
        <button key=${s.id}
                class=${`mob-tab ${e===s.id?"on":""}`}
                data-testid=${`mobile-tab-${s.id}`}
                onClick=${()=>D.value=s.id}>
          <span class="mt-ic">${s.icon}</span><span>${s.label}</span>
        </button>
      `)}
    </div>
  `}import{html as Ze}from"htm/preact";import{useState as ma,useEffect as fa,useMemo as ga,useRef as ha}from"preact/hooks";function Qt(){let e=oe.value,[s,t]=ma(""),a=ha(null);if(fa(()=>{e&&(t(""),setTimeout(()=>a.current?.focus(),0))},[e]),!e)return null;let i=()=>oe.value=!1,{sessions:n}=L.value,o=ga(()=>{let c=[{id:"cmd-fleet",sec:"COMMANDS",label:"Open Fleet",tool:"\u25A6",run:()=>{D.value="fleet",i()}},{id:"cmd-terminal",sec:"COMMANDS",label:"Open Terminal",tool:"\u203A_",run:()=>{D.value="terminal",i()}},{id:"cmd-costs",sec:"COMMANDS",label:"Costs dashboard",tool:"$",run:()=>{D.value="costs",i()}},{id:"cmd-search",sec:"COMMANDS",label:"Session search",tool:"/",run:()=>{D.value="search",i()}},{id:"cmd-archived",sec:"COMMANDS",label:"Archived sessions",tool:"\u2302",run:()=>{D.value="archived",i()}},{id:"cmd-tweaks",sec:"COMMANDS",label:"Open Tweaks",tool:"T",run:()=>{X.value=!0,i()}},{id:"cmd-shortcuts",sec:"COMMANDS",label:"Keyboard shortcuts",tool:"?",run:()=>{ee.value=!0,i()}},{id:"cmd-settings",sec:"COMMANDS",label:"Settings drawer",tool:"S",run:()=>{ne.value=!0,i()}}];return M.value&&c.unshift({id:"cmd-new",sec:"COMMANDS",label:"New session",tool:"n",run:()=>{G.value=!0,i()}}),c},[]),l=n.map(c=>({id:c.id,sec:"SESSIONS",label:c.title,tool:c.tool||c.kind,run:()=>{w.value=c.id,D.value="terminal",i()}})),d=[...o,...l].filter(c=>!s||c.label.toLowerCase().includes(s.toLowerCase())),r={};return d.forEach(c=>{(r[c.sec]||=[]).push(c)}),Ze`
    <div class="overlay" onClick=${i}>
      <div class="cmdk" data-testid="command-palette" onClick=${c=>c.stopPropagation()}>
        <div class="inp">
          <${k} d=${x.search}/>
          <input ref=${a} data-testid="palette-input" value=${s} onInput=${c=>t(c.target.value)}
                 placeholder="Type a command or session name…"
                 onKeyDown=${c=>{c.key==="Escape"&&i()}}/>
          <span class="kbd">esc</span>
        </div>
        <div class="list">
          ${Object.entries(r).map(([c,h])=>Ze`
            <div key=${c}>
              <div class="sec">${c}</div>
              ${h.map((m,S)=>Ze`
                <div key=${m.id} data-testid=${m.sec==="SESSIONS"?"palette-session-row":"palette-cmd-row"} class=${`row ${S===0&&c===Object.keys(r)[0]?"f":""}`} onClick=${m.run}>
                  <span>${m.label}</span>
                  <span class="tool">${m.tool||""}</span>
                </div>
              `)}
            </div>
          `)}
          ${d.length===0&&Ze`
            <div data-testid="palette-empty" style="padding: 16px; font-family: var(--mono); font-size: 12px; color: var(--muted); text-align: center;">
              No matches.
            </div>
          `}
        </div>
      </div>
    </div>
  `}import{html as gt}from"htm/preact";var $a=[{id:"blue",color:"var(--tn-blue)"},{id:"amber",color:"var(--tn-yellow)"},{id:"green",color:"var(--tn-green)"},{id:"purple",color:"var(--tn-purple)"}];function Zt(){if(!X.value)return null;let e=we.value,s=Ce.value,t=H.value,a=()=>X.value=!1;return gt`
    <div class="tweaks" role="dialog" aria-label="Tweaks" data-testid="tweaks-panel">
      <div class="th">
        <${k} d=${x.settings} size=${14}/>
        <div class="t">Tweaks</div>
        <button class="icon-btn" data-testid="tweaks-close" onClick=${a} aria-label="Close tweaks">
          <${k} d=${x.x}/>
        </button>
      </div>
      <div class="tb">
        <div>
          <label>ACCENT</label>
          <div class="swatch-row">
            ${$a.map(i=>gt`
              <div key=${i.id}
                   data-testid=${`tweaks-accent-${i.id}`}
                   class=${`swatch ${e===i.id?"on":""}`}
                   style=${{background:i.color}}
                   onClick=${()=>we.value=i.id}/>
            `)}
          </div>
        </div>
        <div>
          <label>DENSITY</label>
          <div class="seg-row">
            ${["compact","balanced","comfortable"].map(i=>gt`
              <button key=${i}
                      data-testid=${`tweaks-density-${i}`}
                      class=${`seg-btn ${s===i?"on":""}`}
                      onClick=${()=>Ce.value=i}>${i}</button>
            `)}
          </div>
        </div>
        <div>
          <label>RIGHT RAIL</label>
          <div style="display: flex; align-items: center; gap: 8px;">
            <div class=${`switch ${t==="visible"?"on":""}`}
                 data-testid="tweaks-rail-switch"
                 onClick=${()=>H.value=t==="visible"?"hidden":"visible"}/>
            <span style="font-family: var(--mono); font-size: 11px; color: var(--text-dim);">
              ${t==="visible"?"visible":"hidden"}
            </span>
          </div>
        </div>
      </div>
    </div>
  `}import{html as Ta}from"htm/preact";import{html as et}from"htm/preact";import{useEffect as ss,useRef as as,useCallback as ba,useState as ns}from"preact/hooks";import{Terminal as ya}from"@xterm/xterm";import{FitAddon as ka}from"@xterm/addon-fit";import{WebglAddon as xa}from"@xterm/addon-webgl";import{html as es}from"htm/preact";function ts(){let t=de.value.filter(i=>i.type==="session"&&i.session).length,a=M.value;return es`
    <div style="flex: 1; min-height: 0; display: flex; align-items: center; justify-content: center; padding: 32px;">
      <div data-testid="empty-state-dashboard"
           style="display: flex; flex-direction: column; align-items: center; gap: 18px; max-width: 420px; text-align: center;">
        <${ge}/>
        <div>
          <div style="font-size: 16px; font-weight: 600; color: var(--text-hi); margin-bottom: 6px;">
            No session selected
          </div>
          <div style="font-family: var(--mono); font-size: 12px; color: var(--muted); line-height: 1.55;">
            ${t===0?"Your deck is empty. Create a session to get started, or browse the fleet view from the sidebar.":`You have ${t} session${t===1?"":"s"}. Pick one from the sidebar, or open the Fleet tab.`}
          </div>
        </div>
        <div style="display: flex; gap: 8px;">
          <button class="btn ghost" onClick=${()=>D.value="fleet"}>
            Open Fleet
          </button>
          ${a&&es`
            <button class="btn primary" onClick=${()=>G.value=!0}>
              <${k} d=${x.plus} size=${12}/>New session <span class="kbd">n</span>
            </button>
          `}
        </div>
      </div>
    </div>
  `}function Sa(){return typeof window.matchMedia=="function"&&window.matchMedia("(pointer: coarse)").matches}function wa(e,s){let t=window.location.protocol==="https:"?"wss":"ws",a=new URL(t+"://"+window.location.host+"/ws/session/"+encodeURIComponent(e));return s&&a.searchParams.set("token",s),a.toString()}function Ca(e,s,t){if(!e||!s)return;let a=!1,i=0;function n(d){!d.touches||d.touches.length!==1||(a=!0,i=d.touches[0].clientY)}function o(d){if(!a||!d.touches||d.touches.length!==1)return;d.preventDefault();let r=d.touches[0].clientY,c=i-r;i=r,s&&c!==0&&s.dispatchEvent(new WheelEvent("wheel",{deltaY:c,deltaMode:0,bubbles:!0,cancelable:!0}))}function l(){a=!1}e.addEventListener("touchstart",n,{capture:!0,passive:!0,signal:t.signal}),e.addEventListener("touchmove",o,{capture:!0,passive:!1,signal:t.signal}),e.addEventListener("touchend",l,{capture:!0,passive:!0,signal:t.signal}),e.addEventListener("touchcancel",l,{capture:!0,passive:!0,signal:t.signal})}function os(){let e=as(null),s=as(null),t=w.value,[a,i]=ns(null),[n,o]=ns(0);ss(()=>(window.__preactTerminalActive=!0,()=>{window.__preactTerminalActive=!1}),[]);let l=ba(()=>{let r=s.current;r&&(r.reconnectTimer&&clearTimeout(r.reconnectTimer),r.ws&&(r.ws.close(),r.ws=null),r.resizeObserver&&r.resizeObserver.disconnect(),r.controller&&r.controller.abort(),r.terminal&&r.terminal.dispose(),s.current=null,Z.value="disconnected")},[]);if(ss(()=>{if(!e.current||!t){l();return}if(s.current&&s.current.sessionId===t&&s.current.reconnectKey===n)return;l(),i(null);let r=e.current,c=se.value,h=Sa(),m=new ya({convertEol:!1,cursorBlink:!h,disableStdin:!1,fontFamily:"IBM Plex Mono, Menlo, Consolas, monospace",fontSize:13,scrollback:1e4,theme:{background:"#0a1220",foreground:"#d9e2ec",cursor:"#9ecbff"}}),S=new ka;m.loadAddon(S),m.open(r);try{let g=new xa;g.onContextLoss(()=>{g.dispose(),typeof window.CanvasAddon<"u"&&m.loadAddon(new window.CanvasAddon.CanvasAddon)}),m.loadAddon(g)}catch{if(typeof window.CanvasAddon<"u")try{m.loadAddon(new window.CanvasAddon.CanvasAddon)}catch{}}r.offsetWidth&&r.offsetHeight&&S.fit();let C=new AbortController;if(!h&&typeof document<"u"){let g=document.createElement("link");g.rel="preload",g.as="script",g.crossOrigin="anonymous",g.href="/static/vendor/addon-webgl.mjs",document.head.appendChild(g),C.signal.addEventListener("abort",()=>{g.parentNode&&g.parentNode.removeChild(g)})}let f={sessionId:t,reconnectKey:n,terminal:m,fitAddon:S,ws:null,resizeObserver:null,controller:C,decoder:new TextDecoder,reconnectTimer:null,reconnectAttempt:0,wsReconnectEnabled:!0,terminalAttached:!1};s.current=f;let I=null;function v(g){clearTimeout(I),I=setTimeout(()=>{if(!r.offsetWidth||!r.offsetHeight)return;S.fit();let{cols:p,rows:F}=m;p>=10&&F>=3&&f.ws&&f.ws.readyState===WebSocket.OPEN&&f.terminalAttached&&f.ws.send(JSON.stringify({type:"resize",cols:p,rows:F}))},g)}if(typeof ResizeObserver=="function"){let g=new ResizeObserver(()=>v(90));g.observe(r),f.resizeObserver=g}window.addEventListener("resize",()=>v(120),{signal:C.signal}),Ca(r,m.element,C);let u=m.onData(g=>{!f.ws||f.ws.readyState!==WebSocket.OPEN||!f.terminalAttached||We.value||f.ws.send(JSON.stringify({type:"input",data:g}))});h||r.addEventListener("paste",g=>{if(We.value||!f.ws||f.ws.readyState!==WebSocket.OPEN||!f.terminalAttached)return;let p=g.clipboardData;if(!p)return;let F=p.getData("text/plain");F&&(F=F.replace(/\r\n?/g,`
`),g.preventDefault(),g.stopPropagation(),f.ws.send(JSON.stringify({type:"input",data:F})))},{capture:!0,signal:C.signal}),m.writeln("Connecting to terminal...");function T(g){let p=Math.min(g,8);return Math.min(8e3,Math.round(350*Math.pow(1.8,p-1)))}function $(){if(!f.wsReconnectEnabled||f.reconnectTimer||f.ws)return;f.reconnectAttempt+=1;let g=T(f.reconnectAttempt);Z.value="connecting",f.reconnectTimer=setTimeout(()=>{f.reconnectTimer=null,E(!0)},g)}function E(g){f.ws&&(f.ws.close(),f.ws=null),f.terminalAttached=!1,f.wsReconnectEnabled=!0,Z.value="connecting";let p=new WebSocket(wa(t,c));p.binaryType="arraybuffer",f.ws=p;function F(){f.ws===p&&(f.reconnectTimer&&(clearTimeout(f.reconnectTimer),f.reconnectTimer=null),f.reconnectAttempt=0,Z.value="connected",p.send(JSON.stringify({type:"ping"})))}function j(fe){if(f.ws===p){if(typeof fe.data=="string"){try{let z=JSON.parse(fe.data);if(z.type==="status")z.event==="connected"?(We.value=!!z.readOnly,m&&(m.options.disableStdin=!!z.readOnly),Z.value="connected"):z.event==="terminal_attached"?(f.terminalAttached=!0,v(0)):z.event==="session_closed"&&(f.terminalAttached=!1);else if(z.type==="error"){if((z.code==="TERMINAL_ATTACH_FAILED"||z.code==="TMUX_SESSION_NOT_FOUND")&&(f.terminalAttached=!1),z.code==="TMUX_SESSION_NOT_FOUND"){f.wsReconnectEnabled=!1,i({code:z.code,message:z.message||"tmux session is not available",hint:z.hint||""}),Z.value="disconnected";return}m.write(`\r
[error:`+(z.code||"unknown")+"] "+(z.message||"unknown error")+`\r
`)}}catch{}return}if(fe.data instanceof ArrayBuffer){let z=f.decoder.decode(new Uint8Array(fe.data),{stream:!0});m.write(z)}}}function J(){f.ws===p&&(Z.value="error")}function Ue(){if(f.ws===p){if(f.ws=null,f.terminalAttached=!1,f.wsReconnectEnabled){$();return}Z.value="disconnected"}}p.addEventListener("open",F,{signal:C.signal}),p.addEventListener("message",j,{signal:C.signal}),p.addEventListener("error",J,{signal:C.signal}),p.addEventListener("close",Ue,{signal:C.signal})}return E(!1),h||m.focus(),()=>{u.dispose(),clearTimeout(I),l()}},[t,n,l]),!t)return et`<${ts} />`;async function d(){try{await b("POST","/api/sessions/"+t+"/restart"),i(null),o(r=>r+1)}catch{}}return et`
    <div class="term-frame" style="position: relative;">
      <div class="term-strip">
        <span class="tdots"><i/><i/><i/></span>
        <span class="tpath">session · ${t}</span>
        <span style="flex: 1;"/>
      </div>
      <div style="flex: 1; min-height: 0; min-width: 0; overflow: hidden; padding: 14px 16px;">
        <div ref=${e} style="height: 100%; width: 100%; overflow: hidden;"/>
      </div>
      ${a&&et`
        <div role="alert"
             style=${{position:"absolute",inset:"12px 12px auto 12px",border:"1px solid rgba(247,118,142,0.4)",background:"rgba(22,22,30,0.95)",borderRadius:"var(--radius-lg)",boxShadow:"0 30px 60px -20px rgba(0,0,0,0.55)",padding:"14px 16px"}}>
          <div style="display: flex; align-items: flex-start; gap: 12px;">
            <span style="color: var(--tn-red); font-size: 18px; line-height: 1;">⚠</span>
            <div style="flex: 1; min-width: 0;">
              <div style="font-weight: 600; color: var(--text-hi);">Terminal disconnected</div>
              <div style="font-size: 12.5px; color: var(--text); margin-top: 4px;">${a.message}</div>
              ${a.hint&&et`<div style="font-size: 11.5px; color: var(--muted); margin-top: 6px;">${a.hint}</div>`}
              <div style="display: flex; gap: 8px; margin-top: 10px;">
                <button type="button" class="btn primary" onClick=${d}>Restart session</button>
                <button type="button" class="btn ghost" onClick=${()=>i(null)}>Dismiss</button>
              </div>
            </div>
          </div>
        </div>
      `}
    </div>
  `}function is(){return Ta`
    <div class="term-wrap">
      <${os}/>
    </div>
  `}import{html as Aa}from"htm/preact";import{html as ht}from"htm/preact";import{useEffect as $t,useRef as tt,useState as bt}from"preact/hooks";var Ne=null;function Ea(){return typeof window>"u"?Promise.reject(new Error("no window")):window.Chart?Promise.resolve(window.Chart):Ne||(Ne=new Promise((e,s)=>{let t=document.createElement("script");t.src="/static/chart.umd.min.js",t.async=!0,t.onload=()=>{window.Chart?e(window.Chart):s(new Error("chart.umd.min.js loaded but window.Chart missing"))},t.onerror=()=>{Ne=null,s(new Error("failed to load chart.umd.min.js"))},document.head.appendChild(t)}),Ne)}var ls=new Intl.NumberFormat(navigator.language,{style:"currency",currency:"USD"});function st(e){return ls.format(e||0)}function Ia(){let e=getComputedStyle(document.documentElement),s=(t,a)=>e.getPropertyValue(t).trim()||a;return{text:s("--chart-text","#6b7280"),grid:s("--chart-grid","#e5e7eb"),legend:s("--chart-legend","#374151"),primary:s("--chart-primary","#2959aa"),primaryFill:s("--chart-primary-fill","rgba(41, 89, 170, 0.1)"),categorical:[s("--chart-categorical-1","#7aa2f7"),s("--chart-categorical-2","#bb9af7"),s("--chart-categorical-3","#7dcfff"),s("--chart-categorical-4","#9ece6a"),s("--chart-categorical-5","#e0af68"),s("--chart-categorical-6","#f7768e"),s("--chart-categorical-7","#73daca"),s("--chart-categorical-8","#ff9e64")]}}function rs(){let[e,s]=bt(null),[t,a]=bt(null),[i,n]=bt(!0),o=tt(null),l=tt(null),d=tt(null),r=tt(null);return $t(()=>{b("GET","/api/costs/summary").then(c=>{s(c),n(!1)}).catch(c=>{a(c.message||"Failed to load cost data"),n(!1)})},[]),$t(()=>{if(i||t||!o.current||!l.current)return;let c=!1;async function h(){try{let[S,C,f]=await Promise.all([Ea(),b("GET","/api/costs/daily?days=30"),b("GET","/api/costs/models")]);if(c||(d.current&&(d.current.destroy(),d.current=null),r.current&&(r.current.destroy(),r.current=null),!o.current||!l.current))return;let I=Ia(),v=C||[],u=v.map(p=>p.date.slice(5)),T=v.map(p=>p.cost_usd);d.current=new S(o.current,{type:"line",data:{labels:u,datasets:[{label:"Daily Cost ($)",data:T,borderColor:I.primary,backgroundColor:I.primaryFill,fill:!0,tension:.3}]},options:{responsive:!0,plugins:{legend:{display:!1}},scales:{x:{ticks:{color:I.text},grid:{color:I.grid}},y:{ticks:{color:I.text,callback:p=>ls.format(p||0)},grid:{color:I.grid}}}}});let $=f||{},E=Object.keys($),g=Object.values($);r.current=new S(l.current,{type:"doughnut",data:{labels:E,datasets:[{data:g,backgroundColor:I.categorical.slice(0,E.length)}]},options:{responsive:!0,plugins:{legend:{position:"bottom",labels:{color:I.legend,font:{size:11}}}}}})}catch{}}h();let m=new MutationObserver(()=>{h()});return m.observe(document.documentElement,{attributes:!0,attributeFilter:["class"]}),()=>{c=!0,m.disconnect()}},[i,t]),$t(()=>()=>{d.current&&(d.current.destroy(),d.current=null),r.current&&(r.current.destroy(),r.current=null)},[]),i?ht`
      <div style="padding: 18px; font-family: var(--mono); font-size: 12px; color: var(--muted);">
        Loading cost data…
      </div>
    `:t?ht`
      <div class="chart-card" style="margin: 14px;">
        <div class="title">Cost tracking unavailable</div>
        <div style="font-family: var(--mono); font-size: 12px; color: var(--text-dim); line-height: 1.6;">
          Start agent-deck with the cost tracker enabled to see spend, daily history, and per-model
          breakdowns here. The fixture binary intentionally runs without it.
        </div>
      </div>
    `:ht`
    <div style="display: flex; flex-direction: column; gap: 12px; flex: 1; min-height: 0; overflow: auto;">
      <div class="stat-grid">
        <div class="stat">
          <div class="lab">TODAY</div>
          <div class="val">${st(e.today_usd)}</div>
          <div class="delta">${e.today_events} events</div>
        </div>
        <div class="stat">
          <div class="lab">THIS WEEK</div>
          <div class="val">${st(e.week_usd)}</div>
          <div class="delta">${e.week_events} events</div>
        </div>
        <div class="stat">
          <div class="lab">THIS MONTH</div>
          <div class="val">${st(e.month_usd)}</div>
          <div class="delta">${e.month_events} events</div>
        </div>
        <div class="stat">
          <div class="lab">PROJECTED</div>
          <div class="val">${st(e.projected_usd)}</div>
          <div class="delta">based on 7-day avg</div>
        </div>
      </div>
      <div style="display: grid; grid-template-columns: 2fr 1fr; gap: 12px;">
        <div class="chart-card">
          <div class="title">Daily spend · last 30 days</div>
          <canvas ref=${o}></canvas>
        </div>
        <div class="chart-card">
          <div class="title">Cost by model</div>
          <canvas ref=${l}></canvas>
        </div>
      </div>
    </div>
  `}function cs(){return Aa`
    <div class="costs">
      <${rs}/>
    </div>
  `}import{html as ve}from"htm/preact";import{useMemo as Oa}from"preact/hooks";function Pa({name:e,items:s,onSelect:t}){let a=s.filter(l=>l.status==="running").length,i=s.filter(l=>l.status==="waiting").length,n=s.filter(l=>l.status==="error").length,o=n?"error":i?"waiting":a?"running":"";return ve`
    <div class=${`group-card ${o}`} data-testid="fleet-group-card" data-group-name=${e}>
      <div class="gc-head">
        <span class="t">${e}</span>
        <span class="health"><span class=${`d ${o||"idle"}`}/></span>
        <span class="cost"></span>
      </div>
      <div class="gc-tiles">
        ${s.slice(0,6).map(l=>ve`
          <button key=${l.id} class="tile" data-testid="fleet-session-tile" data-session-id=${l.id} onClick=${()=>t(l.id)}>
            <span class=${`tdot ${l.status}`}/>
            <span class="tn">${l.title}</span>
            ${l.tool&&ve`<span class="ttool">${l.tool}</span>`}
          </button>
        `)}
      </div>
      <div class="gc-foot">
        <span class="cn"><span class="d running"/>${a}</span>
        <span class="cn"><span class="d waiting"/>${i}</span>
        <span class="cn"><span class="d error"/>${n}</span>
        <span class="path" data-testid="fleet-group-session-count">${s.length} session${s.length===1?"":"s"}</span>
      </div>
    </div>
  `}function ds(){let{groups:e,byGroup:s,sessions:t}=L.value,a=Oa(()=>({running:t.filter(o=>o.status==="running").length,waiting:t.filter(o=>o.status==="waiting").length,error:t.filter(o=>o.status==="error").length,idle:t.filter(o=>o.status==="idle").length}),[t]),i=t.reduce((o,l)=>o+(l.cost||0),0),n=o=>{w.value=o,D.value="terminal"};return ve`
    <div class="fleet" data-testid="fleet-pane">
      <div class="fleet-stats">
        <div class="stat" data-testid="fleet-stat-running"><div class="lbl">RUNNING</div><div class="num running">${a.running}</div></div>
        <div class="stat" data-testid="fleet-stat-waiting"><div class="lbl">WAITING</div><div class="num waiting">${a.waiting}</div></div>
        <div class="stat" data-testid="fleet-stat-error"><div class="lbl">ERROR</div><div class="num error">${a.error}</div></div>
        <div class="stat" data-testid="fleet-stat-idle"><div class="lbl">IDLE</div><div class="num idle">${a.idle}</div></div>
        <div class="stat" data-testid="fleet-stat-cost"><div class="lbl">SPEND · TODAY</div><div class="num cost">$${i.toFixed(2)}</div></div>
        <div class="stat" data-testid="fleet-stat-sessions"><div class="lbl">SESSIONS</div><div class="num">${t.length}</div></div>
      </div>

      <div class="fleet-section">
        <div class="fleet-section-head">
          <span class="kicker">GROUPS</span>
          <span class="sub-kicker">${e.length} group${e.length===1?"":"s"} · ${t.length} session${t.length===1?"":"s"}</span>
        </div>
        ${e.length===0||t.length===0?ve`<div style="font-family: var(--mono); font-size: 11px; color: var(--muted); padding: 16px;">
              No sessions yet. Use the sidebar to create one.
            </div>`:ve`<div class="fleet-grid">
              ${e.map(o=>{let l=s[o.path]||[];return l.length===0?null:ve`<${Pa} key=${o.path} name=${o.label} items=${l} onSelect=${n}/>`})}
            </div>`}
      </div>
    </div>
  `}import{html as U}from"htm/preact";import{useState as Ie,useEffect as Fa,useRef as za}from"preact/hooks";async function at({text:e,target:s,context:t}){let a=(e||"").trim();if(!a)return null;let i=s||"maestro",n="cc-pending-"+Date.now();Ut({correlationId:n,target:i,text:a,stage:"received",ack:"got it\u2026",reply:"",at:new Date().toISOString()});let o;try{o=await b("POST","/api/command-center/ask",{target:i,text:a,context:t||{}})}catch(d){return Se(n,{stage:"failed",ack:"sent, but delivery failed: "+(d.message||"error")}),n}let l=o.correlationId||n;return Se(n,{correlationId:l,target:o.routedTo||i,stage:o.stage||"routed",ack:o.ack||"got it \u2014 routed to "+(o.routedTo||i)}),us(l,o.routedTo||i),l}async function us(e,s,t=0){if(!s||t>6)return;let a=2500+t*1500;setTimeout(async()=>{try{let i="?correlationId="+encodeURIComponent(e)+"&target="+encodeURIComponent(s),n=await b("GET","/api/command-center/reply"+i);if(n&&n.reply){Se(e,{stage:"result",reply:n.reply});return}}catch{}us(e,s,t+1)},a)}function nt(e){return e.kind==="decision"?{text:`re ${e.id||e.question?.slice(0,24)}: `,target:e.route||"conductor-agent-deck",context:{decisionId:e.id||""}}:e.kind==="session"?{text:`re session ${e.title}: `,target:e.conductorTarget||"maestro",context:{sessionTitle:e.title,project:e.project||""}}:{text:`re ${e.name}: `,target:e.target||"conductor-"+e.name,context:{project:e.name}}}import{html as yt}from"htm/preact";var Ma={received:"received",routed:"routed","session-created":"session created",result:"replied",failed:"delivery failed"};function Da(e){return e==="result"?"\u2705":e==="session-created"?"\u{1F680}":e==="failed"?"\u26A0\uFE0F":e==="received"?"\u23F3":"\u{1F4E8}"}function ot(){let e=ae.value;return!e||e.length===0?null:yt`
    <section class="ccd-sec cc-acks" data-testid="cc-acks">
      <h2>🧾 Your asks</h2>
      ${e.map(s=>yt`
        <div class="cc-ack" key=${s.correlationId} data-testid="cc-ack" data-stage=${s.stage}>
          <div class="cc-ack-line">
            <span class="cc-ack-ico">${Da(s.stage)}</span>
            <span class="cc-ack-text" title=${s.text}>${s.text}</span>
          </div>
          <div class="cc-ack-meta">
            got it → ${s.target} → <span class="cc-ack-stage">${Ma[s.stage]||s.stage}</span>
          </div>
          ${s.reply&&yt`<div class="cc-ack-reply" data-testid="cc-ack-reply">${s.reply}</div>`}
        </div>
      `)}
    </section>
  `}import{html as _}from"htm/preact";import{useState as it,useEffect as vs}from"preact/hooks";var ps={running:"\u{1F7E2}",waiting:"\u{1F7E1}",idle:"\u26AA",error:"\u{1F534}",stopped:"\u26AB",absent:"\u26AB"};function Na(e){if(!e)return"";let s=Date.parse(e);if(isNaN(s))return"";let t=Math.max(0,Math.floor((Date.now()-s)/1e3));return t<60?t+"s ago":t<3600?Math.floor(t/60)+"m ago":t<86400?Math.floor(t/3600)+"h ago":Math.floor(t/86400)+"d ago"}function La(e,s){vs(()=>{let t=!1;if(!e){xe.value=null;return}return b("GET","/api/command-center/detail/"+encodeURIComponent(e)).then(a=>{t||(xe.value=a)}).catch(()=>{t||(xe.value=null)}),()=>{t=!0}},[e,s])}function Ra({doc:e}){let[s,t]=it(!0);return _`
    <div class="ccd-doc" data-testid="ccd-doc">
      <button class="ccd-doc-head" onClick=${()=>t(a=>!a)}>
        <span class="ccd-doc-caret">${s?"\u25BE":"\u25B8"}</span>
        <span class="ccd-doc-title">${e.title||e.name}</span>
        <span class="ccd-doc-when">${Na(e.updatedAt)}</span>
      </button>
      ${s&&_`<div class="ccd-doc-body md" dangerouslySetInnerHTML=${{__html:e.html}}></div>`}
    </div>
  `}function ms(){let e=ke.value,s=te.value,t=xe.value,a=M.value,[i,n]=it(""),[o,l]=it(!1),[d,r]=it("ready");La(e,s&&s.generatedAt);let c=()=>{ke.value=null,xe.value=null};vs(()=>{let $=E=>{E.key==="Escape"&&(document.activeElement&&document.activeElement.tagName==="TEXTAREA"?document.activeElement.blur():c()),E.key==="/"&&document.activeElement?.tagName!=="TEXTAREA"&&(E.preventDefault(),document.querySelector(".ccd-input textarea")?.focus())};return window.addEventListener("keydown",$),()=>window.removeEventListener("keydown",$)},[]);let h=t&&t.target||"conductor-"+e,m=async $=>{let E=i.trim();if(!(!E||o)){if(!a){A("Two-way input is disabled (web mutations off)","info");return}l(!0),r("sending\u2026");try{await at({text:E,target:h,context:$||{project:e}}),r("\u2713 routed to "+h),n("")}catch(g){r("\u2717 "+(g.message||"send failed"))}finally{l(!1)}}},S=$=>{let E=nt($);n(E.text),document.querySelector(".ccd-input textarea")?.focus()},C=$=>{$.key==="Enter"&&($.ctrlKey||$.metaKey)&&($.preventDefault(),m())},f=t&&t.sessions||[],I=t&&t.docs||[],v=t&&t.decisions||[],u=t&&t.inProgress||[],T=t&&t.recentlyDone||[];return _`
    <div class="ccd" data-testid="command-center-detail-pane">
      <div class="ccd-top">
        <button class="ccd-back" onClick=${c} data-testid="ccd-back" title="Back (Esc)">← back</button>
        <span class="ccd-dot">${ps[t&&t.status]||"\u26AA"}</span>
        <h1>${e}</h1>
        <span class="ccd-headline" data-testid="ccd-headline">${t&&t.headline||""}</span>
      </div>

      ${!t&&_`<div class="cc-empty" data-testid="ccd-loading">loading ${e}…</div>`}

      ${t&&_`
        <div class="ccd-grid">
          <div class="ccd-main">
            ${(u.length>0||T.length>0)&&_`
              <section class="ccd-sec">
                ${u.length>0&&_`
                  <h2>🛠 In progress</h2>
                  <ul class="ccd-list">${u.map(($,E)=>_`<li key=${E}>${$}</li>`)}</ul>`}
                ${T.length>0&&_`
                  <h2>✅ Recently done</h2>
                  <ul class="ccd-list">${T.map(($,E)=>_`<li key=${E}>${$}</li>`)}</ul>`}
              </section>`}

            <section class="ccd-sec" data-testid="ccd-docs">
              <h2>📄 Produced docs ${I.length?_`<span class="ccd-count">${I.length}</span>`:""}</h2>
              ${I.length?I.map($=>_`<${Ra} key=${$.name} doc=${$}/>`):_`<div class="cc-sdone" data-testid="ccd-no-docs">no docs yet — drops here from ${e}'s outputs/</div>`}
            </section>
          </div>

          <div class="ccd-side">
            <section class="ccd-sec">
              <h2>👉 Decisions</h2>
              ${v.length?v.map(($,E)=>_`
                    <div class="cc-ask" key=${$.id||E}>
                      ${$.id&&_`<span class="cc-ask-id">${$.id}</span>`}
                      <span class="cc-ask-text">${$.question}</span>
                      <button class="cc-cmt" title="Answer this"
                        onClick=${()=>S({kind:"decision",...$})}>💬</button>
                    </div>`):_`<div class="cc-sdone">none waiting</div>`}
            </section>

            <section class="ccd-sec" data-testid="ccd-sessions">
              <h2>🛰️ Live sessions</h2>
              ${f.length?f.map($=>_`
                    <div class="cc-srow" key=${$.id} data-testid="ccd-session" data-status=${$.status}>
                      <span class="cc-sd">${ps[$.status]||"\u26AA"}</span>
                      <span class="cc-stt" title=${$.workingOn||$.title}>${$.title}</span>
                      <button class="cc-cmt" title="Comment on this session"
                        onClick=${()=>S({kind:"session",title:$.title,conductorTarget:h,project:e})}>💬</button>
                    </div>`):_`<div class="cc-sdone">no active sessions</div>`}
            </section>

            <${ot}/>
          </div>
        </div>

        <div class="ccd-input cc-input" data-testid="ccd-input">
          <span class="ccd-scope" title="This input routes to ${h}">→ ${h}</span>
          <textarea
            placeholder=${"talk to "+e+"\u2026 \u2318/Ctrl+Enter to send, Esc to go back"}
            value=${i}
            onInput=${$=>n($.target.value)}
            onKeyDown=${C}></textarea>
          <button class="cc-send" disabled=${!i.trim()||o} onClick=${()=>m()} data-testid="ccd-send">➤ Send</button>
          <span class=${`cc-st ${d.startsWith("\u2713")?"ok":d.startsWith("\u2717")?"err":""}`} data-testid="ccd-status">${d}</span>
        </div>
      `}
    </div>
  `}var fs={running:"\u{1F7E2}",waiting:"\u{1F7E1}",idle:"\u26AA",error:"\u{1F534}",stopped:"\u26AB",absent:"\u26AB"},gs={"model-unavailable":"model unavailable","auth-401":"auth error (401)","idle-at-empty-prompt":"idle (empty prompt)"};function _a(e,s){return e==="maestro"&&s==="running"?"\u{1F535}":fs[s]||"\u26AA"}function Ua(e){return e?["running","waiting","idle"].filter(s=>e[s]).map(s=>`${e[s]} ${s}`).join(" \xB7 "):""}function Ga({decision:e,onComment:s}){return U`
    <div class="cc-ask" data-testid="cc-decision">
      ${e.id&&U`<span class="cc-ask-id">${e.id}</span>`}
      <span class="cc-ask-text">${e.question}</span>
      <button class="cc-cmt" title="Comment / answer this"
        onClick=${()=>s({kind:"decision",...e})}>💬</button>
    </div>
  `}function ja({sess:e,conductorTarget:s,project:t,onComment:a}){let i=e.substate&&gs[e.substate];return U`
    <div class="cc-srow" data-testid="cc-session" data-status=${e.status}>
      <span class="cc-sd">${fs[e.status]||"\u26AA"}</span>
      <span class="cc-stt" title=${e.workingOn||e.title}>${e.title}</span>
      ${i&&U`<span class="cc-sub" title=${"honest-status: "+e.substate}>${i}</span>`}
      <button class="cc-cmt" title="Comment on this session"
        onClick=${()=>a({kind:"session",title:e.title,conductorTarget:s,project:t})}>💬</button>
    </div>
  `}function Wa({cd:e,index:s,focused:t,onComment:a,onOpen:i}){let[n,o]=Ie(!1),l=e.substate&&gs[e.substate];return U`
    <div class=${`cc-cd ${n?"open":""} ${t?"focused":""}`}
      data-testid="cc-conductor" data-name=${e.name}>
      <div class="cc-cd-head">
        <button class="cc-cd-toggle" title="Expand sessions" onClick=${()=>o(d=>!d)}>
          <span class="cc-jump">${s<9?s+1:""}</span>
          <span class="cc-dot">${_a(e.name,e.status)}</span>
          <span class="cc-nm">${e.name}</span>
          <span class="cc-ac" title=${e.currentlyWorkingOn||""}>
            ${e.currentlyWorkingOn||(e.status==="absent"?"no conductor session":e.status)}
            ${l&&U` · ${l}`}
          </span>
          <span class="cc-lc">${Ua(e.counts)}</span>
          ${e.docCount>0&&U`<span class="cc-docs" title=${e.docCount+" docs"}>📄${e.docCount}</span>`}
        </button>
        <button class="cc-cmt" title="Comment on this project"
          onClick=${()=>a({kind:"conductor",name:e.name,target:e.target})}>💬</button>
        <button class="cc-open" title="Open detail page" data-testid="cc-open-detail"
          onClick=${()=>i(e.name)}>open →</button>
      </div>
      ${n&&U`
        <div class="cc-cd-body">
          ${e.sessions&&e.sessions.length?e.sessions.map(d=>U`<${ja} key=${d.id} sess=${d}
                conductorTarget=${e.target} project=${e.name} onComment=${a}/>`):U`<div class="cc-sdone">no active sessions</div>`}
        </div>
      `}
    </div>
  `}function hs(){if(ke.value)return U`<${ms}/>`;let e=te.value,s=Y.value,t=M.value,[a,i]=Ie(""),[n,o]=Ie("maestro"),[l,d]=Ie(!1),[r,c]=Ie("ready"),[h,m]=Ie(-1),S=za([]),C=e&&Array.isArray(e.conductors)?e.conductors:[];S.current=C;let f=g=>{ke.value=g};Fa(()=>{let g=p=>{let F=document.activeElement?.tagName==="TEXTAREA"||document.activeElement?.tagName==="INPUT"||document.activeElement?.tagName==="SELECT";if(p.key==="/"&&!F){p.preventDefault(),document.querySelector(".cc-input textarea")?.focus();return}if(p.key==="Escape"&&F){document.activeElement.blur();return}if(F)return;let j=S.current;if(p.key==="ArrowDown")p.preventDefault(),m(J=>Math.min(j.length-1,J+1));else if(p.key==="ArrowUp")p.preventDefault(),m(J=>Math.max(0,J-1));else if(p.key==="Enter"&&h>=0&&j[h])p.preventDefault(),f(j[h].name);else if(/^[1-9]$/.test(p.key)){let J=parseInt(p.key,10)-1;j[J]&&(p.preventDefault(),m(J),f(j[J].name))}};return window.addEventListener("keydown",g),()=>window.removeEventListener("keydown",g)},[h]);let I=g=>{let p=nt(g);i(p.text),p.target&&o(p.target),document.querySelector(".cc-input textarea")?.focus()},v=async()=>{let g=a.trim();if(!(!g||l)){if(!t){A("Two-way input is disabled (web mutations off)","info");return}d(!0),c("sending\u2026");try{await at({text:g,target:n}),c("\u2713 routed to "+n),i("")}catch(p){c("\u2717 "+(p.message||"send failed"))}finally{d(!1)}}},u=g=>{g.key==="Enter"&&(g.ctrlKey||g.metaKey)&&(g.preventDefault(),v())},T=e&&Array.isArray(e.askTargets)?e.askTargets:["maestro"];if(!e)return U`
      <div class="cc" data-testid="command-center-pane">
        <div class="cc-top">
          <h1>Command Center</h1>
          <span class=${`cc-live ${s==="connected"?"":"stale"}`}>
            ${s==="connected"?"\u25CF connecting\u2026":"\u25CF offline"}
          </span>
        </div>
        <div class="cc-empty" data-testid="cc-loading">Waiting for the first fleet snapshot…</div>
      </div>
    `;let $=Array.isArray(e.decisionsWaiting)?e.decisionsWaiting:[],E=e.totals||{};return U`
    <div class="cc" data-testid="command-center-pane">
      <div class="cc-top">
        <h1>Command Center</h1>
        <span class=${`cc-live ${s==="connected"?"":"stale"}`} data-testid="cc-live">
          ${s==="connected"?"\u25CF live":"\u25CF offline"}
        </span>
        <span class="cc-hint">↑↓ move · Enter / 1–9 open · / type · Esc back</span>
        <span class="cc-totals" data-testid="cc-totals">
          ${E.running||0} running · ${E.waiting||0} waiting · ${E.idle||0} idle
        </span>
      </div>

      <div class="cc-cols">
        <div class="cc-col">
          <h2>👉 Needs you</h2>
          ${$.length?$.map((g,p)=>U`<${Ga} key=${g.id||p} decision=${g} onComment=${I}/>`):U`<div class="cc-sdone" data-testid="cc-no-decisions">nothing waiting on you 🎉</div>`}
          <${ot}/>
        </div>

        <div class="cc-col">
          <h2>🛰️ The fleet — what each is doing</h2>
          ${C.length?C.map((g,p)=>U`<${Wa} key=${g.name} cd=${g} index=${p}
                focused=${p===h} onComment=${I} onOpen=${f}/>`):U`<div class="cc-sdone" data-testid="cc-no-conductors">no conductors detected</div>`}
        </div>
      </div>

      <div class="cc-input" data-testid="cc-input">
        <select value=${n} onChange=${g=>o(g.target.value)} title="Route to" data-testid="cc-target">
          ${T.map(g=>U`<option key=${g} value=${g==="conductor-maestro"?"maestro":g}>
            ${g==="conductor-maestro"||g==="maestro"?"Maestro (default)":g}
          </option>`)}
        </select>
        <textarea
          placeholder="answer a decision, comment, or instruct… ⌘/Ctrl+Enter to send"
          value=${a}
          onInput=${g=>i(g.target.value)}
          onKeyDown=${u}></textarea>
        <button class="cc-send" disabled=${!a.trim()||l} onClick=${v} data-testid="cc-send">➤ Send</button>
        <span class=${`cc-st ${r.startsWith("\u2713")?"ok":r.startsWith("\u2717")?"err":""}`} data-testid="cc-status">${r}</span>
      </div>
    </div>
  `}import{html as kt}from"htm/preact";import{useState as Ha,useMemo as $s,useEffect as Ka}from"preact/hooks";function Ba(e){let s=e||{};return{id:s.id||"",title:s.title||s.id,tool:s.tool||"",status:s.status||"idle",group:s.groupPath||"",path:s.projectPath||"",archivedAt:s.archivedAt||null}}function qa(e){if(!e)return"\u2014";try{return new Date(e).toLocaleString(void 0,{dateStyle:"medium",timeStyle:"short"})}catch{return String(e)}}function bs(e){w.value===e&&(w.value=null,typeof window<"u"&&window.location.pathname.startsWith("/s/")&&history.replaceState(null,"","/"))}function ys(){let e=je.value||[],[s,t]=Ha("");Ka(()=>{qe()},[]);let a=$s(()=>e.map(Ba),[e]),i=$s(()=>{if(!s)return a;let l=s.toLowerCase();return a.filter(d=>((d.title||"")+" "+(d.path||"")+" "+(d.tool||"")+" "+(d.group||"")).toLowerCase().includes(l))},[a,s]),n=l=>{if(!M.value){A("mutations disabled");return}b("POST",`/api/sessions/${l.id}/unarchive`).then(()=>{bs(l.id),qe(),A(`Unarchived "${l.title}"`,"success")}).catch(()=>{})},o=l=>{if(!M.value){A("mutations disabled");return}W.value={message:`Delete archived session "${l.title}"? This removes it permanently.`,onConfirm:()=>b("DELETE",`/api/sessions/${l.id}`).then(()=>{bs(l.id),qe()}).catch(()=>{})}};return kt`
    <div class="search-wrap archived-wrap">
      <div class="field">
        <label>ARCHIVED SESSIONS</label>
        <input placeholder="Filter by title, path, tool, group…"
               value=${s} onInput=${l=>t(l.target.value)}/>
      </div>
      <div style="font-family: var(--mono); font-size: 10.5px; color: var(--muted); letter-spacing: 0.08em;">
        ${i.length} ARCHIVED · unarchive to return to the active list
      </div>
      ${i.length===0&&kt`
        <div class="archived-empty">No archived sessions.</div>
      `}
      ${i.map(l=>kt`
        <div key=${l.id} class="sr archived-row">
          <div class="sr-h">
            <${Ge} status=${l.status}/>
            <span class="s">${l.title}</span>
            <span class="w">${l.tool||"\u2014"} · archived ${qa(l.archivedAt)}</span>
          </div>
          <div class="sr-b">${l.path||l.group||""}</div>
          <div class="archived-actions" onClick=${d=>d.stopPropagation()}>
            <button class="mini good" title="Unarchive" onClick=${()=>n(l)}>Unarchive</button>
            <button class="mini danger" title="Delete" onClick=${()=>o(l)}>Delete</button>
          </div>
        </div>
      `)}
    </div>
  `}import{html as ks}from"htm/preact";function xt({title:e,message:s,hotkey:t}){return ks`
    <div class="costs">
      <div class="chart-card" style="display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; gap: 14px; padding: 48px 24px; min-height: 320px;">
        <${ge}/>
        <div class="title" style="font-size: 16px;">${e}</div>
        <div style="font-family: var(--mono); font-size: 12px; color: var(--text-dim); line-height: 1.6; max-width: 460px;">
          ${s}
        </div>
        <div style="font-family: var(--mono); font-size: 11px; color: var(--muted); padding-top: 8px;">
          No data yet — see TUI for now${t?" ":"."}
          ${t&&ks`<span class="kbd" style="border:1px solid var(--border); padding: 1px 6px; border-radius: 3px; color: var(--text); margin-left: 4px;">${t}</span>`}
        </div>
      </div>
    </div>
  `}import{html as xs}from"htm/preact";import{useState as Ja,useMemo as Va}from"preact/hooks";function Ss(){let{sessions:e}=L.value,[s,t]=Ja(""),a=Va(()=>{if(!s)return e;let n=s.toLowerCase();return e.filter(o=>((o.title||"")+" "+(o.path||"")+" "+(o.tool||"")+" "+(o.group||"")).toLowerCase().includes(n))},[e,s]),i=n=>{w.value=n,D.value="terminal"};return xs`
    <div class="search-wrap" data-testid="search-pane">
      <div class="field">
        <label>SESSION SEARCH</label>
        <input autofocus placeholder="Search sessions by title, path, tool, group…"
               data-testid="search-input"
               value=${s} onInput=${n=>t(n.target.value)}/>
      </div>
      <div data-testid="search-result-count" style="font-family: var(--mono); font-size: 10.5px; color: var(--muted); letter-spacing: 0.08em;">
        ${a.length} MATCH${a.length===1?"":"ES"} · cross-profile search not exposed via web API
      </div>
      ${a.map(n=>xs`
        <div key=${n.id} class="sr" data-testid="search-result" data-session-id=${n.id} onClick=${()=>i(n.id)}>
          <div class="sr-h">
            <span class="s">${n.title}</span>
            <span class="w">${n.tool||"\u2014"} · ${n.status}</span>
          </div>
          <div class="sr-b">${n.path||n.group||""}</div>
        </div>
      `)}
    </div>
  `}import{html as B}from"htm/preact";import{useEffect as Ya,useState as lt,useCallback as Xa}from"preact/hooks";var rt=["local","global","user"];async function Le(e,s={}){let t=await fetch(e,{headers:{"Content-Type":"application/json"},...s});if(!t.ok){let a=`${t.status} ${t.statusText}`;try{let i=await t.json();i&&i.error&&(a=i.error)}catch{}throw new Error(a)}return t.status===204?null:t.json()}function ws(){let{sessions:e}=L.value,s=w.value,t=M.value,a=e.find(v=>v.id===s),[i,n]=lt([]),[o,l]=lt({local:[],global:[],user:[]}),[d,r]=lt(!1),[c,h]=lt(""),m=Xa(async()=>{if(a){r(!0),h("");try{let[v,u]=await Promise.all([Le("/api/mcps"),Le(`/api/sessions/${encodeURIComponent(a.id)}/mcps`)]);n(v.mcps||[]),l({local:u.local||[],global:u.global||[],user:u.user||[]})}catch(v){h(v.message)}finally{r(!1)}}},[a&&a.id]);Ya(()=>{m()},[m]);let S=v=>{for(let u of rt)if(o[u].includes(v))return u;return null},C=async(v,u)=>{if(a)try{await Le(`/api/sessions/${encodeURIComponent(a.id)}/mcps/${encodeURIComponent(v)}`,{method:"POST",body:JSON.stringify({scope:u})}),A(`Attached ${v} (${u})`,"success"),await m()}catch(T){A(`Attach failed: ${T.message}`,"error")}},f=async v=>{if(!a)return;let u=S(v);try{await Le(`/api/sessions/${encodeURIComponent(a.id)}/mcps/${encodeURIComponent(v)}`,{method:"DELETE",body:u?JSON.stringify({scope:u}):""}),A(`Detached ${v}`,"success"),await m()}catch(T){A(`Detach failed: ${T.message}`,"error")}},I=async(v,u)=>{if(a)try{await Le(`/api/sessions/${encodeURIComponent(a.id)}/mcps/${encodeURIComponent(v)}`,{method:"PATCH",body:JSON.stringify({scope:u})}),A(`Moved ${v} \u2192 ${u}`,"success"),await m()}catch(T){A(`Move failed: ${T.message}`,"error")}};return a?B`
    <div class="costs" data-testid="mcp-pane">
      <div class="chart-card" style="padding: 24px;">
        <div class="title" style="font-size: 16px; margin-bottom: 4px;">MCP Manager</div>
        <div style="font-family: var(--mono); font-size: 11px; color: var(--text-dim); margin-bottom: 16px;">
          ${a.title} · ${a.path||""}
        </div>

        ${c&&B`
          <div style="font-family: var(--mono); font-size: 11px; color: var(--err); background: var(--err-bg); padding: 8px 12px; border-radius: 4px; margin-bottom: 12px;" data-testid="mcp-error">
            ${c}
          </div>
        `}

        <div style="display: grid; grid-template-columns: 1fr; gap: 24px;">
          <${Qa}
            attached=${o}
            mutationsEnabled=${t}
            onDetach=${f}
            onMove=${I}/>

          <${Za}
            catalog=${i}
            attached=${o}
            mutationsEnabled=${t}
            onAttach=${C}
            loading=${d}/>
        </div>
      </div>
    </div>
  `:B`
      <div class="costs">
        <div class="chart-card" style="text-align: center; padding: 48px 24px;">
          <div class="title" style="font-size: 16px;">MCP Manager</div>
          <div style="font-family: var(--mono); font-size: 12px; color: var(--text-dim); padding-top: 8px;">
            Select a session in the sidebar to manage MCPs.
          </div>
        </div>
      </div>
    `}function Qa({attached:e,mutationsEnabled:s,onDetach:t,onMove:a}){let i=rt.flatMap(n=>e[n].map(o=>({name:o,scope:n})));return B`
    <div data-testid="mcp-attached">
      <div style="font-family: var(--mono); font-size: 11px; color: var(--muted); letter-spacing: 0.08em; margin-bottom: 8px;">
        ATTACHED (${i.length})
      </div>
      ${i.length===0&&B`
        <div style="font-family: var(--mono); font-size: 12px; color: var(--text-dim); padding: 12px;">
          No MCPs attached. Use the catalog below to attach.
        </div>
      `}
      ${i.map(({name:n,scope:o})=>B`
        <div key=${`${o}-${n}`} data-testid=${`mcp-attached-${n}`}
             style="display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; border: 1px solid var(--border); border-radius: 4px; margin-bottom: 6px;">
          <div>
            <span style="font-family: var(--mono); font-size: 13px; color: var(--text);">${n}</span>
            <span style="font-family: var(--mono); font-size: 10px; color: var(--muted); margin-left: 8px; letter-spacing: 0.08em;">
              ${o.toUpperCase()}
            </span>
          </div>
          <div style="display: flex; gap: 6px;">
            <select disabled=${!s}
                    data-testid=${`mcp-scope-${n}`}
                    value=${o}
                    onChange=${l=>a(n,l.target.value)}
                    style="font-family: var(--mono); font-size: 11px; background: var(--bg); color: var(--text); border: 1px solid var(--border); padding: 2px 6px; border-radius: 3px;">
              ${rt.map(l=>B`<option value=${l} key=${l}>${l}</option>`)}
            </select>
            <button disabled=${!s}
                    data-testid=${`mcp-detach-${n}`}
                    onClick=${()=>t(n)}
                    style="font-family: var(--mono); font-size: 11px; background: transparent; color: var(--err); border: 1px solid var(--err); padding: 2px 8px; border-radius: 3px; cursor: pointer;">
              Detach
            </button>
          </div>
        </div>
      `)}
    </div>
  `}function Za({catalog:e,attached:s,mutationsEnabled:t,onAttach:a,loading:i}){let n=o=>rt.some(l=>s[l].includes(o));return B`
    <div data-testid="mcp-catalog">
      <div style="font-family: var(--mono); font-size: 11px; color: var(--muted); letter-spacing: 0.08em; margin-bottom: 8px;">
        CATALOG (${e.length})
      </div>
      ${i&&B`<div style="font-family: var(--mono); font-size: 11px; color: var(--text-dim); padding: 8px;">Loading…</div>`}
      ${!i&&e.length===0&&B`
        <div style="font-family: var(--mono); font-size: 12px; color: var(--text-dim); padding: 12px;">
          No MCPs in the catalog. Add some to <code>~/.agent-deck/config.toml</code>.
        </div>
      `}
      ${e.map(o=>{let l=n(o.name);return B`
          <div key=${o.name} data-testid=${`mcp-catalog-${o.name}`}
               style="display: flex; align-items: center; justify-content: space-between; padding: 8px 12px; border: 1px solid var(--border); border-radius: 4px; margin-bottom: 6px;">
            <div style="display: flex; flex-direction: column;">
              <span style="font-family: var(--mono); font-size: 13px; color: var(--text);">${o.name}</span>
              ${o.description&&B`<span style="font-family: var(--mono); font-size: 11px; color: var(--text-dim); margin-top: 2px;">${o.description}</span>`}
              <span style="font-family: var(--mono); font-size: 10px; color: var(--muted); margin-top: 2px; letter-spacing: 0.06em;">
                ${(o.transport||"stdio").toUpperCase()}${o.command?` \xB7 ${o.command}`:""}
              </span>
            </div>
            <button disabled=${!t||l}
                    data-testid=${`mcp-attach-${o.name}`}
                    onClick=${()=>a(o.name,"local")}
                    style="font-family: var(--mono); font-size: 11px; background: ${l?"transparent":"var(--accent)"}; color: ${l?"var(--muted)":"var(--bg)"}; border: 1px solid ${l?"var(--border)":"var(--accent)"}; padding: 4px 12px; border-radius: 3px; cursor: ${l?"default":"pointer"};">
              ${l?"Attached":"Attach"}
            </button>
          </div>
        `})}
    </div>
  `}import{html as Q}from"htm/preact";import{useEffect as en,useState as ct}from"preact/hooks";var tn=new Set(["claude","gemini","codex","pi"]);function Cs(){let e=w.value,{sessions:s}=L.value,t=s.find(v=>v.id===e),[a,i]=ct([]),[n,o]=ct([]),[l,d]=ct(!1),[r,c]=ct("");async function h(){d(!0);try{let v=await b("GET","/api/skills");if(i(v?.skills||[]),t){let u=await b("GET",`/api/sessions/${encodeURIComponent(t.id)}/skills`);o(u?.skills||[])}else o([])}catch(v){A("Failed to load skills: "+(v.message||"request failed"))}finally{d(!1)}}if(en(()=>{h()},[t&&t.id]),!t)return Q`
      <div class="costs">
        <div class="chart-card" style="padding: 32px; text-align: center;">
          <div class="title">No session selected</div>
          <div style="color: var(--text-dim); margin-top: 12px;">
            Pick a session from the sidebar to manage its skills.
          </div>
        </div>
      </div>`;if(!tn.has((t.tool||"").toLowerCase()))return Q`
      <div class="costs">
        <div class="chart-card" style="padding: 32px; text-align: center;">
          <div class="title">Skills not supported for ${t.tool}</div>
          <div style="color: var(--text-dim); margin-top: 12px;">
            Project-scoped skills are available for Claude, Gemini, Codex, and Pi sessions only.
          </div>
        </div>
      </div>`;let S=new Set(n.map(v=>v.id)),C=a.filter(v=>!S.has(v.id)&&(v.kind||"dir")==="dir");async function f(v){if(!r){c(v.id);try{let u=`/api/sessions/${encodeURIComponent(t.id)}/skills/${encodeURIComponent(v.name)}?source=${encodeURIComponent(v.source)}`;await b("POST",u),A(`Attached ${v.name}`),await h()}catch{}finally{c("")}}}async function I(v){if(!r){c(v.id);try{let u=`/api/sessions/${encodeURIComponent(t.id)}/skills/${encodeURIComponent(v.name)}?source=${encodeURIComponent(v.source)}`;await b("DELETE",u),A(`Detached ${v.name}`),await h()}catch{}finally{c("")}}}return Q`
    <div class="skills-pane" data-testid="skills-pane" style="padding: 16px; display: flex; flex-direction: column; gap: 16px; height: 100%; overflow: auto;">
      <div style="display: flex; justify-content: space-between; align-items: center;">
        <div class="title" style="font-size: 14px;">Skills · ${t.title}</div>
        <button class="btn" data-testid="skills-refresh" onClick=${h} disabled=${l}>${l?"Loading\u2026":"Refresh"}</button>
      </div>

      <section data-testid="skills-attached" style="border: 1px solid var(--border); border-radius: 6px; padding: 12px;">
        <div style="font-family: var(--mono); font-size: 12px; color: var(--text-dim); margin-bottom: 8px;">
          ATTACHED (${n.length})
        </div>
        ${n.length===0?Q`<div data-testid="skills-attached-empty" style="color: var(--muted); font-size: 12px;">No skills attached.</div>`:Q`<ul style="list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px;">
              ${n.map(v=>Q`
                <li data-testid="skill-attached-row" data-skill-id=${v.id} style="display: flex; justify-content: space-between; gap: 8px; align-items: center; padding: 6px 8px; background: var(--surface); border-radius: 4px;">
                  <span><strong>${v.name}</strong> <span style="color: var(--muted); font-size: 11px;">${v.source}</span></span>
                  <button class="btn btn-danger" data-testid="skill-detach-btn" disabled=${r===v.id} onClick=${()=>I(v)}>Detach</button>
                </li>`)}
            </ul>`}
      </section>

      <section data-testid="skills-catalog" style="border: 1px solid var(--border); border-radius: 6px; padding: 12px;">
        <div style="font-family: var(--mono); font-size: 12px; color: var(--text-dim); margin-bottom: 8px;">
          CATALOG (${C.length})
        </div>
        ${C.length===0?Q`<div data-testid="skills-catalog-empty" style="color: var(--muted); font-size: 12px;">No additional skills available to attach.</div>`:Q`<ul style="list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px;">
              ${C.map(v=>Q`
                <li data-testid="skill-catalog-row" data-skill-id=${v.id} style="display: flex; justify-content: space-between; gap: 8px; align-items: center; padding: 6px 8px;">
                  <span>
                    <strong>${v.name}</strong>
                    <span style="color: var(--muted); font-size: 11px;"> ${v.source}</span>
                    ${v.description&&Q`<div style="color: var(--text-dim); font-size: 11px;">${v.description}</div>`}
                  </span>
                  <button class="btn" data-testid="skill-attach-btn" disabled=${r===v.id} onClick=${()=>f(v)}>Attach</button>
                </li>`)}
            </ul>`}
      </section>
    </div>
  `}import{html as re}from"htm/preact";import{useState as me}from"preact/hooks";var Ts=["claude","codex","gemini","opencode","shell"],sn={codex:"ChatGPT"};function dt(e){return sn[e]||e}function an(e){return e.filter((s,t,a)=>a.indexOf(s)===t)}function Es(e){let s=e.length>0?e:Ts;return an(s)}function Is(e,s){let a=[...e.length>0?e:Ts];return s&&!a.includes(s)&&a.push(s),a}var St="__custom__",nn={claude:[{value:"claude-sonnet-4-6",label:"Claude Sonnet 4.6"},{value:"claude-opus-4-8",label:"Claude Opus 4.8"},{value:"claude-opus-4-7",label:"Claude Opus 4.7"},{value:"claude-haiku-4-5",label:"Claude Haiku 4.5 alias"},{value:"claude-haiku-4-5-20251001",label:"Claude Haiku 4.5 pinned"}],codex:[{value:"gpt-5.5",label:"GPT-5.5"},{value:"gpt-5.5-pro",label:"GPT-5.5 Pro"},{value:"gpt-5.4",label:"GPT-5.4"},{value:"gpt-5.4-pro",label:"GPT-5.4 Pro"},{value:"gpt-5.4-mini",label:"GPT-5.4 Mini"},{value:"gpt-5.4-nano",label:"GPT-5.4 Nano"},{value:"gpt-5.3-codex",label:"GPT-5.3 Codex"},{value:"gpt-5.2",label:"GPT-5.2"},{value:"gpt-5.2-pro",label:"GPT-5.2 Pro"},{value:"gpt-5.1",label:"GPT-5.1"},{value:"gpt-5-pro",label:"GPT-5 Pro"},{value:"gpt-5",label:"GPT-5"},{value:"gpt-5-mini",label:"GPT-5 Mini"},{value:"gpt-5-nano",label:"GPT-5 Nano"},{value:"gpt-4.1",label:"GPT-4.1"},{value:"gpt-4.1-mini",label:"GPT-4.1 Mini"},{value:"gpt-4o",label:"GPT-4o"},{value:"gpt-4o-mini",label:"GPT-4o Mini"},{value:"o3-pro",label:"o3 Pro"},{value:"o3",label:"o3"}],gemini:[{value:"gemini-3.1-pro-preview",label:"Gemini 3.1 Pro preview"},{value:"gemini-3.1-pro-preview-customtools",label:"Gemini 3.1 Pro custom tools"},{value:"gemini-3-flash-preview",label:"Gemini 3 Flash preview"},{value:"gemini-3.1-flash-lite",label:"Gemini 3.1 Flash Lite"},{value:"gemini-3.1-flash-lite-preview",label:"Gemini 3.1 Flash Lite preview"},{value:"gemini-2.5-pro",label:"Gemini 2.5 Pro"},{value:"gemini-2.5-flash",label:"Gemini 2.5 Flash"},{value:"gemini-2.5-flash-lite",label:"Gemini 2.5 Flash Lite"}],opencode:[{value:"openai/gpt-5.5",label:"OpenAI GPT-5.5"},{value:"openai/gpt-5.5-pro",label:"OpenAI GPT-5.5 Pro"},{value:"openai/gpt-5.4",label:"OpenAI GPT-5.4"},{value:"openai/gpt-5.4-pro",label:"OpenAI GPT-5.4 Pro"},{value:"openai/gpt-5.4-mini",label:"OpenAI GPT-5.4 Mini"},{value:"openai/gpt-5.3-codex",label:"OpenAI GPT-5.3 Codex"},{value:"openai/gpt-5",label:"OpenAI GPT-5"},{value:"openai/o3",label:"OpenAI o3"},{value:"anthropic/claude-sonnet-4-6",label:"Anthropic Claude Sonnet 4.6"},{value:"anthropic/claude-opus-4-8",label:"Anthropic Claude Opus 4.8"},{value:"anthropic/claude-opus-4-7",label:"Anthropic Claude Opus 4.7"},{value:"anthropic/claude-haiku-4-5",label:"Anthropic Claude Haiku 4.5"}]};function on(e){return nn[e]||[]}function As(){let[e,s]=me(""),[t,a]=me("claude"),[i,n]=me(""),[o,l]=me(""),[d,r]=me(""),[c,h]=me(null),[m,S]=me(!1);if(!M.value)return null;async function C(p){p.preventDefault(),h(null),S(!0);try{let F={title:e,tool:t,projectPath:d},j=I();j&&(F.modelId=j),await b("POST","/api/sessions",F),G.value=!1}catch(F){h(F.message)}finally{S(!1)}}function f(p){a(p),n(""),l("")}function I(){return i===St?o.trim():i||""}let v=()=>G.value=!1,u=p=>{p.target===p.currentTarget&&v()},T=on(t),$=Es(ye.value),E=i===St,g=m||!e||!d||E&&!o.trim();return re`
    <div class="overlay" onClick=${u}>
      <form class="dialog" onClick=${p=>p.stopPropagation()} onSubmit=${C}>
        <div class="dh">
          <span class="kicker">NEW</span>
          <div class="t">New session</div>
          <button type="button" class="icon-btn" onClick=${v} aria-label="Close">
            <${k} d=${x.x}/>
          </button>
        </div>
        <div class="db">
          <div class="field">
            <label>TITLE</label>
            <input autofocus required value=${e} onInput=${p=>s(p.target.value)} placeholder="my-session"/>
          </div>
          <div class="field">
            <label>WORKING DIR</label>
            <input required value=${d} onInput=${p=>r(p.target.value)} placeholder="/absolute/path/to/project"/>
          </div>
          <div class="field">
            <label>TOOL</label>
            <div class="seg-row">
              ${$.map(p=>re`
                <button type="button" key=${p}
                        class=${`seg-btn ${t===p?"on":""}`}
                        onClick=${()=>f(p)}>${dt(p)}</button>
              `)}
            </div>
            ${He.value&&re`
              <div style="font-family: var(--mono); font-size: 11px; color: var(--tn-comment, #888);
                          margin-top: 6px;">
                No tools matched PATH; showing all. Set <code>show_only_installed_tools = false</code> to silence.
              </div>
            `}
          </div>
          ${T.length>0&&re`
            <div class="field">
              <label>MODEL ID</label>
              <select value=${i} onInput=${p=>n(p.target.value)}>
                <option value="">Tool default</option>
                ${T.map(p=>re`
                  <option key=${p.value} value=${p.value}>${p.value} — ${p.label}</option>
                `)}
                <option value=${St}>Custom model ID…</option>
              </select>
            </div>
            ${E&&re`
              <div class="field">
                <label>MODEL ID</label>
                <input required value=${o} onInput=${p=>l(p.target.value)} placeholder="provider/model-or-version"/>
              </div>
            `}
          `}
          ${c&&re`
            <div style="font-family: var(--mono); font-size: 11.5px; color: var(--tn-red); padding: 8px 10px;
                        border: 1px solid rgba(247,118,142,0.3); border-radius: 4px; background: rgba(247,118,142,0.06);">
              ${c}
            </div>
          `}
        </div>
        <div class="df">
          <button type="button" class="btn ghost" onClick=${v}>Cancel</button>
          <button type="submit" class="btn primary" disabled=${g}>
            ${m?"Creating\u2026":re`Create session <span class="kbd">⏎</span>`}
          </button>
        </div>
      </form>
    </div>
  `}import{html as Re}from"htm/preact";import{useState as q,useMemo as ln}from"preact/hooks";function rn(e,s){let t={};return e.title!==s.title&&(t.title=e.title),e.notes!==(s.notes||"")&&(t.notes=e.notes),e.color!==(s.color||"")&&(t.color=e.color),e.tool!==(s.tool||"")&&(t.tool=e.tool),e.tool==="claude"&&(e.extraArgs!==(s.extraArgs||"")&&(t.extraArgs=e.extraArgs),e.plugins!==(s.plugins||"")&&(t.plugins=e.plugins),e.channels!==(s.channels||"")&&(t.channels=e.channels),e.skipPermissions!==!!s.skipPermissions&&(t.skipPermissions=e.skipPermissions),e.autoMode!==!!s.autoMode&&(t.autoMode=e.autoMode)),t}function Os(){let e=Pe.value,{sessions:s}=L.value,t=ln(()=>e?s.find(O=>O.id===e.sessionId):null,[e&&e.sessionId,s]),a=t||{title:"",notes:"",color:"",tool:"claude"},[i,n]=q(a.title),[o,l]=q(a.notes||""),[d,r]=q(a.color||""),[c,h]=q(a.tool||"claude"),[m,S]=q(a.extraArgs||""),[C,f]=q(a.plugins||""),[I,v]=q(a.channels||""),[u,T]=q(!!a.skipPermissions),[$,E]=q(!!a.autoMode),[g,p]=q(null),[F,j]=q(!1),[J,Ue]=q(e?e.sessionId:null);if(e&&t&&J!==e.sessionId&&(n(t.title||""),l(t.notes||""),r(t.color||""),h(t.tool||"claude"),S(t.extraArgs||""),f(t.plugins||""),v(t.channels||""),T(!!t.skipPermissions),E(!!t.autoMode),p(null),Ue(e.sessionId)),!e||!M.value||!t)return null;let fe=t.tool||"",z=Is(ye.value,fe);async function js(O){O.preventDefault(),p(null);let Et=rn({title:i,notes:o,color:d,tool:c,extraArgs:m,plugins:C,channels:I,skipPermissions:u,autoMode:$},t);if(Object.keys(Et).length===0){Oe();return}j(!0);try{await b("PATCH",`/api/sessions/${encodeURIComponent(t.id)}`,Et),Oe()}catch(It){p(It.message||String(It))}finally{j(!1)}}function Oe(){Pe.value=null,Ue(null)}let Ws=O=>{O.target===O.currentTarget&&Oe()},Hs=F||!i.trim();return Re`
    <div class="overlay" onClick=${Ws} data-testid="edit-session-dialog">
      <form class="dialog" onClick=${O=>O.stopPropagation()} onSubmit=${js}>
        <div class="dh">
          <span class="kicker">EDIT</span>
          <div class="t">Edit session</div>
          <button type="button" class="icon-btn" onClick=${Oe} aria-label="Close">
            <${k} d=${x.x}/>
          </button>
        </div>
        <div class="db">
          <div class="field">
            <label>TITLE</label>
            <input
              autofocus required
              data-testid="edit-session-title"
              value=${i}
              onInput=${O=>n(O.target.value)}
              placeholder="Session title"/>
          </div>
          <div class="field">
            <label>NOTES</label>
            <input
              data-testid="edit-session-notes"
              value=${o}
              onInput=${O=>l(O.target.value)}
              placeholder="Optional notes"/>
          </div>
          <div class="field">
            <label>COLOR</label>
            <input
              data-testid="edit-session-color"
              value=${d}
              onInput=${O=>r(O.target.value)}
              placeholder="#RRGGBB, 0-255, or blank to clear"/>
          </div>
          <div class="field">
            <label>TOOL (restart required)</label>
            <div class="seg-row">
              ${z.map(O=>Re`
                <button type="button" key=${O}
                        class=${`seg-btn ${c===O?"on":""}`}
                        onClick=${()=>h(O)}>${dt(O)}</button>
              `)}
            </div>
          </div>
          ${c==="claude"&&Re`
            <div class="field">
              <label>EXTRA ARGS (restart, claude)</label>
              <input
                data-testid="edit-session-extra-args"
                value=${m}
                onInput=${O=>S(O.target.value)}
                placeholder="--model opus --verbose"/>
            </div>
            <div class="field">
              <label>PLUGINS (restart, claude — comma-separated)</label>
              <input
                data-testid="edit-session-plugins"
                value=${C}
                onInput=${O=>f(O.target.value)}
                placeholder="octopus,discord"/>
            </div>
            <div class="field">
              <label>CHANNELS (restart, claude — comma-separated)</label>
              <input
                data-testid="edit-session-channels"
                value=${I}
                onInput=${O=>v(O.target.value)}
                placeholder="plugin:telegram@org/repo"/>
            </div>
            <div class="field">
              <label>
                <input type="checkbox"
                       data-testid="edit-session-skip-permissions"
                       checked=${u}
                       onChange=${O=>T(O.target.checked)}/>
                Skip permissions (restart, claude)
              </label>
            </div>
            <div class="field">
              <label>
                <input type="checkbox"
                       data-testid="edit-session-auto-mode"
                       checked=${$}
                       onChange=${O=>E(O.target.checked)}/>
                Auto mode (restart, claude)
              </label>
            </div>
          `}
          ${g&&Re`
            <div data-testid="edit-session-error"
                 style="font-family: var(--mono); font-size: 11.5px; color: var(--tn-red); padding: 8px 10px;
                        border: 1px solid rgba(247,118,142,0.3); border-radius: 4px; background: rgba(247,118,142,0.06);">
              ${g}
            </div>
          `}
        </div>
        <div class="df">
          <button type="button" class="btn ghost" onClick=${Oe}>Cancel</button>
          <button type="submit" class="btn primary"
                  data-testid="edit-session-save"
                  disabled=${Hs}>
            ${F?"Saving\u2026":Re`Save <span class="kbd">⏎</span>`}
          </button>
        </div>
      </form>
    </div>
  `}import{html as cn}from"htm/preact";import{useEffect as dn,useRef as un}from"preact/hooks";function Ps({message:e,onConfirm:s}){let t=un(null);dn(()=>{t.current&&t.current.focus()},[]);let a=()=>W.value=null,i=()=>{s(),W.value=null};return cn`
    <div class="overlay" onClick=${o=>o.target===o.currentTarget&&a()}>
      <div role="dialog" aria-modal="true" aria-label="Confirm action"
           class="dialog" style="max-width: 460px;"
           onClick=${o=>o.stopPropagation()}
           onKeyDown=${o=>{o.key==="Escape"&&(o.stopPropagation(),a())}}>
        <div class="dh">
          <span class="kicker" style="color: var(--tn-red); background: rgba(247,118,142,0.12);">CONFIRM</span>
          <div class="t">Are you sure?</div>
          <button type="button" class="icon-btn" onClick=${a} aria-label="Close">
            <${k} d=${x.x}/>
          </button>
        </div>
        <div class="db">
          <div style="font-family: var(--sans); color: var(--text); line-height: 1.55;">${e}</div>
        </div>
        <div class="df">
          <button type="button" class="btn ghost" ref=${t} onClick=${a}>Cancel</button>
          <button type="button" class="btn danger" onClick=${i}>Delete</button>
        </div>
      </div>
    </div>
  `}import{html as Ms}from"htm/preact";import{useState as wt}from"preact/hooks";function Ds({mode:e,groupPath:s,currentName:t,onSubmit:a}){let[i,n]=wt(t||""),[o,l]=wt(null),[d,r]=wt(!1),c=e==="create",h=c?"New group":"Rename group",m=c?"Create":"Rename",S=()=>be.value=null;async function C(f){f.preventDefault(),l(null),r(!0);try{c?await b("POST","/api/groups",{name:i}):await b("PATCH","/api/groups/"+encodeURIComponent(s),{name:i}),be.value=null,a&&a()}catch(I){l(I.message)}finally{r(!1)}}return Ms`
    <div class="overlay" onClick=${f=>f.target===f.currentTarget&&S()}>
      <form class="dialog" style="max-width: 460px;"
            onClick=${f=>f.stopPropagation()}
            onSubmit=${C}>
        <div class="dh">
          <span class="kicker">${c?"NEW":"RENAME"}</span>
          <div class="t">${h}</div>
          <button type="button" class="icon-btn" onClick=${S} aria-label="Close">
            <${k} d=${x.x}/>
          </button>
        </div>
        <div class="db">
          <div class="field">
            <label>NAME</label>
            <input autofocus required value=${i} onInput=${f=>n(f.target.value)} placeholder="my-group"/>
          </div>
          ${o&&Ms`
            <div style="font-family: var(--mono); font-size: 11.5px; color: var(--tn-red); padding: 8px 10px;
                        border: 1px solid rgba(247,118,142,0.3); border-radius: 4px; background: rgba(247,118,142,0.06);">
              ${o}
            </div>
          `}
        </div>
        <div class="df">
          <button type="button" class="btn ghost" onClick=${S}>Cancel</button>
          <button type="submit" class="btn primary" disabled=${d||!i}>
            ${d?c?"Creating\u2026":"Renaming\u2026":m}
          </button>
        </div>
      </form>
    </div>
  `}import{html as Ct}from"htm/preact";import{useState as pn,useEffect as vn}from"preact/hooks";function Ns(){let[e,s]=pn(null),t=mt.value;return vn(()=>{t||fetch("/api/settings").then(a=>{if(!a.ok)throw new Error("Settings request failed: "+a.status);return a.json()}).then(a=>{mt.value=a}).catch(a=>s(a.message||"Failed to load settings"))},[]),e?Ct`<div style="font-family: var(--mono); font-size: 12px; color: var(--tn-red);">${e}</div>`:t?Ct`
    <div data-testid="settings-panel" style="display: flex; flex-direction: column; gap: 2px;">
      <div class="kv" data-testid="settings-profile"><span class="k">profile</span><span class="v">${t.profile||"default"}</span></div>
      <div class="kv" data-testid="settings-version"><span class="k">version</span><span class="v">${t.version||"unknown"}</span></div>
      <div class="kv" data-testid="settings-read-only"><span class="k">read-only</span><span class=${`v ${t.readOnly?"warn":"ok"}`}>${t.readOnly?"yes":"no"}</span></div>
      <div class="kv" data-testid="settings-web-mutations"><span class="k">web mutations</span><span class=${`v ${t.webMutations?"ok":"warn"}`}>${t.webMutations?"enabled":"disabled"}</span></div>
      <div class="kv" data-testid="settings-hidden-tools"><span class="k">hidden tools</span><span class="v">${(t.hiddenTools||[]).join(", ")||"none"}</span></div>
      <div class="kv" data-testid="settings-picker-tools"><span class="k">picker tools</span><span class="v">${(t.pickerTools||[]).join(", ")||"loading\u2026"}</span></div>
      <div style="font-family: var(--mono); font-size: 11px; color: var(--muted); margin-top: 8px;">
        Edit <code>~/.agent-deck/config.toml</code> (<code>[ui] hidden_tools</code>) or use TUI Settings → Visible tools…
      </div>
    </div>
  `:Ct`<div style="font-family: var(--mono); font-size: 12px; color: var(--muted);">Loading…</div>`}import{html as Fe}from"htm/preact";var mn=[{keys:["/"],label:"Focus session filter / search"},{keys:["j"],label:"Move focus down (next session)"},{keys:["k"],label:"Move focus up (previous session)"},{keys:["Enter"],label:"Open focused session"},{keys:["Shift","Enter"],label:"Open focused session in new browser tab"},{keys:["n"],label:"New session dialog"},{keys:["r"],label:"Rename focused session (TUI-only today)"},{keys:["Shift","D"],label:"Close focused session (stop process, keep metadata)"},{keys:["Ctrl","Z"],label:"Undo last delete (within 30s)"},{keys:["q"],label:"Close current modal / overlay"},{keys:["Esc"],label:"Close modal / unfocus input"},{keys:["?"],label:"Toggle this help overlay"},{keys:["Ctrl","K"],label:"Command palette"},{keys:["]"],label:"Toggle right rail"}];function fn({k:e}){return Fe`<span class="kbd kshort-kbd">${e}</span>`}function Ls(){if(!ee.value)return null;let e=()=>ee.value=!1;return Fe`
    <div class="overlay kshort-overlay" role="dialog" aria-label="Keyboard shortcuts"
         data-testid="shortcuts-overlay"
         onClick=${e}>
      <div class="dialog kshort-dialog" onClick=${s=>s.stopPropagation()}>
        <div class="dh">
          <span class="kicker">HELP</span>
          <div class="t">Keyboard shortcuts</div>
          <button class="icon-btn" onClick=${e} aria-label="Close help">
            <${k} d=${x.x}/>
          </button>
        </div>
        <div class="db">
          <table class="kshort-table">
            <tbody>
              ${mn.map(s=>Fe`
                <tr key=${s.keys.join("+")}>
                  <td class="kshort-keys">
                    ${s.keys.map((t,a)=>Fe`
                      ${a>0&&Fe`<span class="kshort-plus">+</span>`}
                      <${fn} k=${t}/>
                    `)}
                  </td>
                  <td class="kshort-label">${s.label}</td>
                </tr>
              `)}
            </tbody>
          </table>
          <div class="kshort-foot">
            Web binds the most-used TUI keys (issue #780). Web-only actions
            (e.g. <span class="kbd">Ctrl</span>+<span class="kbd">K</span>) are
            included for completeness.
          </div>
        </div>
      </div>
    </div>
  `}function gn(){let{sessions:e}=L.value,s=w.value,t=e.find(d=>d.id===s)||e[0];if(!t)return null;let a=(t.kind||"agent").toUpperCase(),i=ie.value||"",n=M.value,o=t.model?`${t.model}${t.modelVersion?` ${t.modelVersion}`:""}`:"",l=d=>{if(n)return d==="fork"?b("POST",`/api/sessions/${t.id}/fork`,{title:t.title+"-fork"}).catch(()=>{}):b("POST",`/api/sessions/${t.id}/${d}`).catch(()=>{})};return R`
    <div class="work-head">
      <div class="path">
        <span class=${`kind ${t.kind||""}`}>${a}</span>
        ${i&&R`<span class="seg">${i} /</span>`}
        <span class="seg">${t.group||"default"} /</span>
        <span class="cur">${t.title}</span>
      </div>
      <span class=${`status-chip ${t.status}`}><span class="d"/>${t.status}</span>
      ${o&&R`<span class="status-chip model" title=${t.modelId||o}>${o}</span>`}
      <span class="spacer"/>
      ${n&&R`
        <div class="actions">
          ${t.status==="running"||t.status==="waiting"?R`<button class="btn ghost" onClick=${()=>l("stop")}><${k} d=${x.stop} size=${12}/>Stop</button>`:R`<button class="btn ghost" onClick=${()=>l("start")}><${k} d=${x.play} size=${12}/>Start</button>`}
          <button class="btn ghost" onClick=${()=>l("restart")}><${k} d=${x.restart} size=${12}/>Restart</button>
          ${t.canFork&&R`<button class="btn" onClick=${()=>l("fork")}><${k} d=${x.fork} size=${12}/>Fork</button>`}
          <button class="btn primary" onClick=${()=>G.value=!0}>
            <${k} d=${x.plus} size=${12}/>New <span class="kbd">n</span>
          </button>
        </div>
      `}
    </div>
  `}function hn({tab:e}){return R`
    <div style=${{display:e==="terminal"?"flex":"none",flex:1,minHeight:0,flexDirection:"column"}}>
      <${is}/>
    </div>
    ${e==="command-center"&&R`<${hs}/>`}
    ${e==="fleet"&&R`<${ds}/>`}
    ${e==="costs"&&R`<${cs}/>`}
    ${e==="search"&&R`<${Ss}/>`}
    ${e==="archived"&&R`<${ys}/>`}
    ${e==="mcp"&&R`<${ws}/>`}
    ${e==="skills"&&R`<${Cs}/>`}
    ${e==="conductor"&&R`<${xt} title="Conductor"
                              message="Conductor orchestration view is TUI-only. The web API does not expose child topology, bridges, or NEED escalation."/>`}
    ${e==="watchers"&&R`<${xt} title="Watchers"
                              message="Watcher framework events are routed in the backend; the web API does not surface event streams or routing config."/>`}
  `}function Rs(){let e=D.value,s=G.value,t=W.value,a=be.value,i=ne.value;return Ae(()=>{let n=document.querySelector("body > .app");return n&&n.id!=="app-root-grid"&&(n.style.display="none"),()=>{n&&(n.style.display="")}},[]),Ae(()=>{fetch("/api/settings").then(n=>n.ok?n.json():null).then(n=>{n&&(typeof n.webMutations=="boolean"&&(M.value=n.webMutations),typeof n.toolFilter=="boolean"&&(Ft.value=n.toolFilter),Array.isArray(n.visibleTools)&&(zt.value=n.visibleTools),typeof n.toolFilterFallback=="boolean"&&(He.value=n.toolFilterFallback),Array.isArray(n.hiddenTools)&&(_t.value=n.hiddenTools),Array.isArray(n.pickerTools)&&n.pickerTools.length>0&&(ye.value=n.pickerTools))}).catch(()=>{})},[]),Ae(()=>{fetch("/api/profiles").then(n=>n.ok?n.json():null).then(n=>{n&&Array.isArray(n.profiles)&&(Ke.value=n,n.current&&(ie.value=n.current))}).catch(()=>{})},[]),Ae(()=>{let n=!1,o=()=>{fetch("/api/system/stats").then(d=>d.ok?d.json():null).then(d=>{!n&&d&&(Be.value=d)}).catch(()=>{})};o();let l=setInterval(o,5e3);return()=>{n=!0,clearInterval(l)}},[]),Ae(()=>{let n=r=>{let c=L.value?.sessions||[];if(c.length===0)return;let h=w.value,m=c.findIndex(C=>C.id===h);m===-1&&(m=r>0?-1:c.length);let S=c[Math.max(0,Math.min(c.length-1,m+r))];S&&(w.value=S.id)},o=()=>{let r=L.value?.sessions||[],c=w.value;return r.find(h=>h.id===c)||r[0]||null},l=()=>{oe.value=!1,X.value=!1,ee.value=!1,G.value=!1,W.value=null,be.value=null,ne.value=!1},d=r=>{let c=r.target,h=c&&(c.tagName==="INPUT"||c.tagName==="TEXTAREA"||c.tagName==="SELECT"||c.isContentEditable);if((r.metaKey||r.ctrlKey)&&r.key.toLowerCase()==="k"){r.preventDefault(),oe.value=!0;return}if(r.key==="Escape"){h&&typeof c.blur=="function"&&c.blur(),l();return}if(!h){if(r.key==="Enter"&&r.shiftKey){let m=o();if(m){r.preventDefault();let S=`${window.location.pathname}#session=${encodeURIComponent(m.id)}`;window.open(S,"_blank","noopener")}return}if(r.key==="?")r.preventDefault(),ee.value=!ee.value;else if(r.key==="/")r.preventDefault(),document.querySelector(".side-filter input")?.focus();else if(r.key==="j")r.preventDefault(),n(1);else if(r.key==="k")r.preventDefault(),n(-1);else if(r.key==="Enter"){let m=o();m&&(r.preventDefault(),w.value=m.id,D.value="terminal")}else if(r.key==="n"&&M.value)G.value=!0;else if(r.key==="r"){let m=o();m&&A(`Rename "${m.title}": use the TUI (web rename API not implemented yet)`,"info")}else if(r.key==="D"){if(!M.value)return;let m=o();if(!m)return;W.value={message:`Close session "${m.title}"? The tmux process will be killed; metadata is preserved.`,onConfirm:()=>b("POST",`/api/sessions/${m.id}/close`).catch(()=>{})}}else if((r.metaKey||r.ctrlKey)&&r.key.toLowerCase()==="z"){if(!M.value)return;r.preventDefault(),b("POST","/api/sessions/undelete").then(m=>{m&&m.sessionId?A(`Restored session ${m.sessionId}`,"success"):A("Restored last deleted session","success")}).catch(()=>A("Nothing to undo","info"))}else r.key==="q"?l():r.key==="]"&&(H.value=H.value==="visible"?"hidden":"visible")}};return window.addEventListener("keydown",d),()=>window.removeEventListener("keydown",d)},[]),Ae(()=>{if(!i)return;let n=o=>{o.key==="Escape"&&(ne.value=!1)};return document.addEventListener("keydown",n),()=>document.removeEventListener("keydown",n)},[i]),R`
    <div id="app-root-grid" class="app">
      <${Ht}/>
      <${Kt}/>
      <div class="main">
        <${gn}/>
        <div class="work-body">
          <${hn} tab=${e}/>
        </div>
      </div>
      <${Vt}/>
      <${Bt}/>
      <${Xt}/>

      ${s&&R`<${As}/>`}
      <${Os}/>
      ${t&&R`<${Ps} ...${t}/>`}
      ${a&&R`<${Ds} ...${a}/>`}

      ${i&&R`
        <div class="overlay" onClick=${()=>ne.value=!1}>
          <div class="dialog" onClick=${n=>n.stopPropagation()}>
            <div class="dh">
              <span class="kicker">SETTINGS</span>
              <div class="t">Settings</div>
              <button class="icon-btn" onClick=${()=>ne.value=!1} aria-label="Close settings">
                <${k} d=${x.x}/>
              </button>
            </div>
            <div class="db">
              <${Ns}/>
            </div>
          </div>
        </div>
      `}

      <${Qt}/>
      <${Zt}/>
      <${Ls}/>
      <${Mt}/>
      <${Wt}/>
    </div>
  `}function zs(){return Fs(()=>{function e(){let s=window.location.pathname||"/";if(s.startsWith("/s/")){let t=s.slice(3);if(t&&!t.includes("/")){try{w.value=decodeURIComponent(t)}catch{w.value=null}return}}s==="/"&&(w.value=null)}return window.addEventListener("popstate",e),()=>window.removeEventListener("popstate",e)},[]),Fs(()=>{let e=w.value,s=window.location.pathname,t=e?"/s/"+encodeURIComponent(e):"/";s!==t&&window.history.pushState(null,"",t)},[w.value]),$n`
    <${Rs} />
  `}(function(){let s=new URLSearchParams(window.location.search),t=s.get("token");if(!t)return;se.value=t,s.delete("token");let a=s.toString(),i=window.location.pathname+(a?"?"+a:"")+window.location.hash;history.replaceState(null,"",i);let n=document.querySelector('meta[name="referrer"]');n||(n=document.createElement("meta"),n.name="referrer",document.head.appendChild(n)),n.content="no-referrer"})();var ze=null;function _s(){if(ze)return;let e=se.value,s=e?"/events/menu?token="+encodeURIComponent(e):"/events/menu",t=new EventSource(s);ze=t,t.addEventListener("menu",a=>{try{let i=JSON.parse(a.data);i&&Array.isArray(i.items)&&(de.value=i.items,ft.value=!0),Y.value="connected"}catch{}}),t.addEventListener("error",()=>{Y.value="disconnected"})}function jr(){ze&&(ze.close(),ze=null),_e&&(_e.close(),_e=null)}var _e=null,ut=new Set;function Us(){if(_e)return;let e=se.value,s=e?"/events/command-center?token="+encodeURIComponent(e):"/events/command-center",t=new EventSource(s);_e=t,t.addEventListener("command-center",a=>{try{let i=JSON.parse(a.data);if(i&&typeof i=="object"){let n=te.value;te.value=i,kn(n,i);let o=Array.isArray(i.recentlyCompleted)?i.recentlyCompleted:[];for(let l of o){let d=(l&&(l.id||""))+":"+(l&&(l.at||""));ut.has(d)||(ut.add(d),l&&l.title&&A(`\u2705 ${l.title} just finished`,"success"))}ut.size>200&&ut.clear()}}catch{}})}function Gs(e){let s=e&&e.totals;return s?(s.running||0)+(s.waiting||0)+(s.idle||0):0}function kn(e,s){if(!e||Gs(s)<=Gs(e))return;let a=ae.value.find(i=>i.stage==="routed");a&&Se(a.correlationId,{stage:"session-created"})}async function xn(){try{let e=await b("GET","/api/menu");de.value=e.items||[],ft.value=!0,_s(),Us()}catch{Y.value="disconnected",_s(),Us()}}function Sn(){let e=window.location.pathname||"/";if(e.startsWith("/s/")){let s=e.slice(3);if(s&&!s.includes("/")){try{w.value=decodeURIComponent(s)}catch{w.value=null}return}}}function wn(){if(!("serviceWorker"in navigator))return;function e(){navigator.serviceWorker.register("/sw.js",{scope:"/"}).catch(()=>{})}document.readyState==="complete"||document.readyState==="interactive"?e():window.addEventListener("load",e,{once:!0})}var Tt=document.getElementById("app-root");Tt&&(Tt.style.cssText="position:fixed;inset:0;z-index:10;",Sn(),xn(),wn(),bn(yn`<${zs} />`,Tt));export{Sn as applyRouteSelection,xn as loadMenu,wn as registerServiceWorker,Us as startCommandCenterSSE,_s as startSSE,jr as stopSSE};
