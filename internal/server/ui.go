package server

import (
	"net/http"
)

// uiHTML is the self-contained dashboard for Brand.
// Served at GET /ui — no build step, no external files.
const uiHTML = `<!DOCTYPE html><html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Brand — Stockyard</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital,wght@0,400;0,700;1,400&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
<style>:root{
  --bg:#1a1410;--bg2:#241e18;--bg3:#2e261e;
  --rust:#c45d2c;--rust-light:#e8753a;--rust-dark:#8b3d1a;
  --leather:#a0845c;--leather-light:#c4a87a;
  --cream:#f0e6d3;--cream-dim:#bfb5a3;--cream-muted:#7a7060;
  --gold:#d4a843;--green:#5ba86e;--red:#c0392b;
  --font-serif:'Libre Baskerville',Georgia,serif;
  --font-mono:'JetBrains Mono',monospace;
}
*{margin:0;padding:0;box-sizing:border-box}
body{background:var(--bg);color:var(--cream);font-family:var(--font-serif);min-height:100vh;overflow-x:hidden}
a{color:var(--rust-light);text-decoration:none}a:hover{color:var(--gold)}
.hdr{background:var(--bg2);border-bottom:2px solid var(--rust-dark);padding:.9rem 1.8rem;display:flex;align-items:center;justify-content:space-between;gap:1rem}
.hdr-left{display:flex;align-items:center;gap:1rem}
.hdr-brand{font-family:var(--font-mono);font-size:.75rem;color:var(--leather);letter-spacing:3px;text-transform:uppercase}
.hdr-title{font-family:var(--font-mono);font-size:1.1rem;color:var(--cream);letter-spacing:1px}
.badge{font-family:var(--font-mono);font-size:.6rem;padding:.2rem .6rem;letter-spacing:1px;text-transform:uppercase;border:1px solid}
.badge-free{color:var(--green);border-color:var(--green)}
.badge-pro{color:var(--gold);border-color:var(--gold)}
.badge-ok{color:var(--green);border-color:var(--green)}
.badge-err{color:var(--red);border-color:var(--red)}
.main{max-width:1000px;margin:0 auto;padding:2rem 1.5rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:1rem;margin-bottom:2rem}
.card{background:var(--bg2);border:1px solid var(--bg3);padding:1.2rem 1.5rem}
.card-val{font-family:var(--font-mono);font-size:1.8rem;font-weight:700;color:var(--cream);display:block}
.card-lbl{font-family:var(--font-mono);font-size:.62rem;letter-spacing:2px;text-transform:uppercase;color:var(--leather);margin-top:.3rem}
.section{margin-bottom:2.5rem}
.section-title{font-family:var(--font-mono);font-size:.68rem;letter-spacing:3px;text-transform:uppercase;color:var(--rust-light);margin-bottom:.8rem;padding-bottom:.5rem;border-bottom:1px solid var(--bg3)}
table{width:100%;border-collapse:collapse;font-family:var(--font-mono);font-size:.78rem}
th{background:var(--bg3);padding:.5rem .8rem;text-align:left;color:var(--leather-light);font-weight:400;letter-spacing:1px;font-size:.65rem;text-transform:uppercase}
td{padding:.5rem .8rem;border-bottom:1px solid var(--bg3);color:var(--cream-dim);vertical-align:top;word-break:break-all}
tr:hover td{background:var(--bg2)}
.empty{color:var(--cream-muted);text-align:center;padding:2rem;font-style:italic}
.btn{font-family:var(--font-mono);font-size:.75rem;padding:.4rem 1rem;border:1px solid var(--leather);background:transparent;color:var(--cream);cursor:pointer;transition:all .2s}
.btn:hover{border-color:var(--rust-light);color:var(--rust-light)}
.btn-rust{border-color:var(--rust);color:var(--rust-light)}.btn-rust:hover{background:var(--rust);color:var(--cream)}
.pill{display:inline-block;font-family:var(--font-mono);font-size:.6rem;padding:.1rem .4rem;border-radius:2px;text-transform:uppercase}
.pill-get{background:#1a3a2a;color:var(--green)}.pill-post{background:#2a1f1a;color:var(--rust-light)}
.pill-del{background:#2a1a1a;color:var(--red)}.pill-ok{background:#1a3a2a;color:var(--green)}
.pill-err{background:#2a1a1a;color:var(--red)}
.mono{font-family:var(--font-mono);font-size:.78rem}
.lbl{font-family:var(--font-mono);font-size:.62rem;letter-spacing:1px;text-transform:uppercase;color:var(--leather)}
.upgrade{background:var(--bg2);border:1px solid var(--rust-dark);border-left:3px solid var(--rust);padding:.8rem 1.2rem;font-size:.82rem;color:var(--cream-dim);margin-bottom:1.5rem}
.upgrade a{color:var(--rust-light)}
pre{background:var(--bg3);padding:.8rem 1rem;font-family:var(--font-mono);font-size:.75rem;color:var(--cream-dim);overflow-x:auto;max-width:100%}
input,select{font-family:var(--font-mono);font-size:.78rem;background:var(--bg3);border:1px solid var(--bg3);color:var(--cream);padding:.4rem .7rem;outline:none}
input:focus,select:focus{border-color:var(--leather)}
.row{display:flex;gap:.8rem;align-items:flex-end;flex-wrap:wrap;margin-bottom:1rem}
.field{display:flex;flex-direction:column;gap:.3rem}
.sserow{padding:.4rem .8rem;border-bottom:1px solid var(--bg3);font-family:var(--font-mono);font-size:.72rem;color:var(--cream-dim);display:grid;grid-template-columns:120px 60px 1fr;gap:.5rem}
.sserow:nth-child(odd){background:var(--bg2)}

.chain-ok{border-left:4px solid var(--green);background:var(--bg2);padding:1rem 1.5rem}
.chain-err{border-left:4px solid var(--red);background:#1a0a0a;padding:1rem 1.5rem}
.chain-val{font-family:var(--font-mono);font-size:.72rem;color:var(--cream-dim);margin-top:.4rem;word-break:break-all}
</style></head><body>
<div class="hdr">
  <div class="hdr-left">
    <svg viewBox="0 0 64 64" width="22" height="22" fill="none"><rect x="8" y="8" width="8" height="48" rx="2.5" fill="#e8753a"/><rect x="28" y="8" width="8" height="48" rx="2.5" fill="#e8753a"/><rect x="48" y="8" width="8" height="48" rx="2.5" fill="#e8753a"/><rect x="8" y="27" width="48" height="7" rx="2.5" fill="#c4a87a"/></svg>
    <span class="hdr-brand">Stockyard</span>
    <span class="hdr-title">Brand</span>
  </div>
  <div style="display:flex;gap:.8rem;align-items:center">
    <span id="tier-badge" class="badge badge-free">Free</span>
    <a href="/api/stats" class="lbl" style="color:var(--leather)">API</a>
    <a href="https://stockyard.dev/brand/" class="lbl" style="color:var(--leather)">Docs</a>
  </div>
</div>
<div class="main">

<div class="cards">
  <div class="card"><span class="card-val" id="s-total">—</span><span class="card-lbl">Total Events</span></div>
  <div class="card"><span class="card-val" id="s-today">—</span><span class="card-lbl">Events Today</span></div>
  <div class="card"><span class="card-val" id="s-head" style="font-size:.9rem;word-break:break-all">—</span><span class="card-lbl">Head Hash</span></div>
</div>

<div style="display:grid;grid-template-columns:1fr 1fr;gap:2rem;margin-bottom:2rem">

  <div class="section">
    <div class="section-title">Chain Integrity</div>
    <div id="chain-status" style="padding:1rem;background:var(--bg2);border:1px solid var(--bg3)">
      <span class="lbl" style="color:var(--leather)">Click verify to check the chain</span>
    </div>
    <button class="btn btn-rust" style="margin-top:.8rem" onclick="verifyChain()">Verify Chain Now</button>
    <div style="margin-top:.6rem;font-size:.78rem;color:var(--cream-dim)">
      <a href="/api/evidence/export" style="font-family:var(--font-mono);font-size:.72rem">&#8659; Export evidence pack</a>
    </div>
  </div>

  <div class="section">
    <div class="section-title">Append Event</div>
    <div class="row">
      <div class="field"><span class="lbl">Type</span><input id="evt-type" placeholder="user_login" style="width:150px"></div>
      <div class="field"><span class="lbl">Actor</span><input id="evt-actor" placeholder="alice" style="width:120px"></div>
    </div>
    <div class="field" style="margin-bottom:.6rem">
      <span class="lbl">Detail (JSON)</span>
      <input id="evt-detail" placeholder='{"ip":"1.2.3.4"}' style="width:100%">
    </div>
    <button class="btn btn-rust" onclick="appendEvent()">Append Event</button>
    <div id="append-result" class="mono" style="margin-top:.5rem;font-size:.72rem;color:var(--green)"></div>
  </div>

</div>

<div class="section">
  <div class="section-title">Event Feed <span class="lbl">(last 50, newest first)</span></div>
  <div class="row">
    <div class="field"><span class="lbl">Type filter</span><input id="type-filter" placeholder="user_login" style="width:150px" oninput="loadEvents()"></div>
    <div class="field"><span class="lbl">Actor filter</span><input id="actor-filter" placeholder="alice" style="width:130px" oninput="loadEvents()"></div>
  </div>
  <table><thead><tr><th>Seq</th><th>Type</th><th>Actor</th><th>Resource</th><th>Hash</th><th>When</th></tr></thead>
  <tbody id="evt-list"><tr><td colspan="6" class="empty">Loading...</td></tr></tbody></table>
</div>

<div class="section">
  <div class="section-title">Policy Templates <span class="lbl">(Pro)</span></div>
  <div style="display:flex;gap:.8rem;flex-wrap:wrap">
    <button class="btn" onclick="applyTemplate('soc2')">SOC2 Type II</button>
    <button class="btn" onclick="applyTemplate('hipaa')">HIPAA</button>
    <button class="btn" onclick="applyTemplate('gdpr')">GDPR Article 30</button>
    <button class="btn" onclick="applyTemplate('eu_ai_act')">EU AI Act</button>
  </div>
  <div id="template-result" class="mono" style="margin-top:.6rem;font-size:.72rem;color:var(--green)"></div>
</div>

</div>
<script>
let _timer=null;
function autoReload(fn,ms=8000){if(_timer)clearInterval(_timer);_timer=setInterval(fn,ms)}
function ts(s){if(!s)return'-';const d=new Date(s);return d.toLocaleString()}
function rel(s){if(!s)return'-';const d=new Date(s),n=new Date(),diff=Math.round((n-d)/1000);if(diff<60)return diff+'s ago';if(diff<3600)return Math.round(diff/60)+'m ago';return Math.round(diff/3600)+'h ago'}
function fmt(n){return n===undefined||n===null?'-':n.toLocaleString()}
function pill(m){const c={'GET':'pill-get','POST':'pill-post','DELETE':'pill-del'}[m]||'';return '<span class="pill '+c+'">'+m+'</span>'}
function status(s){const ok=s>=200&&s<300;return '<span class="pill '+(ok?'pill-ok':'pill-err')+'">'+s+'</span>'}

const API='/api';

async function loadStats(){
  const r=await(await fetch(API+'/stats')).json().catch(()=>({}));
  document.getElementById('s-total').textContent=fmt(r.total_events);
  document.getElementById('s-today').textContent=fmt(r.events_today);
  const h=r.head_hash||'genesis';
  document.getElementById('s-head').textContent=h.length>16?h.slice(0,8)+'…'+h.slice(-6):h;
  document.getElementById('s-head').title=h;
}

async function loadEvents(){
  const type=document.getElementById('type-filter').value.trim();
  const actor=document.getElementById('actor-filter').value.trim();
  let url=API+'/events?limit=50';
  if(type)url+='&type='+encodeURIComponent(type);
  if(actor)url+='&actor='+encodeURIComponent(actor);
  const r=await(await fetch(url)).json().catch(()=>({entries:[]}));
  const evts=r.entries||[];
  document.getElementById('evt-list').innerHTML=evts.length?evts.map(e=>{
    const detail=typeof e.detail==='object'?JSON.stringify(e.detail).slice(0,60):'';
    return ` + "`" + `<tr>
      <td class="mono" style="color:var(--leather)">${e.seq}</td>
      <td style="color:var(--cream)">${e.event_type}</td>
      <td class="mono" style="font-size:.72rem">${e.actor||'—'}</td>
      <td class="mono" style="font-size:.72rem;max-width:120px;overflow:hidden;text-overflow:ellipsis" title="${detail}">${e.resource||detail||'—'}</td>
      <td class="mono" style="font-size:.68rem;color:var(--leather-light)">${e.entry_hash?e.entry_hash.slice(0,12)+'…':'—'}</td>
      <td>${rel(e.created_at)}</td>
    </tr>` + "`" + `;
  }).join(''):'<tr><td colspan="6" class="empty">No events yet. POST to /api/events to start logging.</td></tr>';
}

async function verifyChain(){
  const el=document.getElementById('chain-status');
  el.innerHTML='<span class="lbl">Verifying...</span>';
  const r=await(await fetch(API+'/verify')).json().catch(()=>({valid:false,message:'fetch error'}));
  if(r.valid){
    el.className='chain-ok';
    el.innerHTML='<div style="font-family:var(--font-mono);font-size:.8rem;color:var(--green)">&#10003; Chain intact</div><div class="chain-val">'+r.checked+' entries verified &middot; '+r.message+'</div>';
  }else{
    el.className='chain-err';
    el.innerHTML='<div style="font-family:var(--font-mono);font-size:.8rem;color:var(--red)">&#10007; Chain broken</div><div class="chain-val">'+r.message+'</div>';
  }
}

async function appendEvent(){
  const type=document.getElementById('evt-type').value.trim();
  const actor=document.getElementById('evt-actor').value.trim();
  let detail={};
  try{detail=JSON.parse(document.getElementById('evt-detail').value||'{}')}catch(x){}
  if(!type)return;
  const r=await fetch(API+'/events',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type,actor,detail})}).catch(()=>null);
  if(!r){document.getElementById('append-result').textContent='error';return;}
  const j=await r.json().catch(()=>({}));
  const res=document.getElementById('append-result');
  if(r.ok){
    res.style.color='var(--green)';
    res.textContent='Appended seq='+(j.entry&&j.entry.seq)+' hash='+(j.entry&&j.entry.entry_hash?j.entry.entry_hash.slice(0,12)+'…':'?');
    loadEvents();loadStats();
  }else{res.style.color='var(--red)';res.textContent=j.error||'error';}
}

async function applyTemplate(fw){
  const r=await fetch(API+'/policies/templates/'+fw,{method:'POST'}).catch(()=>null);
  const el=document.getElementById('template-result');
  if(!r){el.style.color='var(--red)';el.textContent='error';return;}
  if(r.status===402){el.style.color='var(--gold)';el.textContent='Policy templates require Pro — upgrade at stockyard.dev/brand/';return;}
  el.style.color='var(--green)';el.textContent=fw+' template applied';
}

async function refresh(){await Promise.all([loadStats(),loadEvents()]);}
refresh();autoReload(refresh,8000);
</script></body></html>`

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write([]byte(uiHTML))
}
