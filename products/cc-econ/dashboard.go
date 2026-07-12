package main

// dashboardHTML is a self-contained page (no external CDN). The server injects
// the report JSON in place of __DATA__ on each request, so a reload re-reads the
// transcripts. No backtick template literals — it lives in a Go raw string.
const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>cc-econ — Claude Code economics</title>
<style>
  :root{
    --bg:#0e1014; --panel:#171a21; --panel2:#12151b; --line:#252b37; --line2:#333b4a;
    --txt:#eaecf2; --dim:#9aa3b5; --mute:#6b7488;
    --accent:#5b9dff; --good:#35c98a; --watch:#f2b34d; --serious:#f2724d;
  }
  *{box-sizing:border-box}
  html{-webkit-font-smoothing:antialiased;text-rendering:optimizeLegibility}
  body{margin:0;min-height:100vh;color:var(--txt);
    background:radial-gradient(1100px 520px at 50% -8%,#151a24,var(--bg)) no-repeat,var(--bg);
    font:14px/1.5 -apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif}
  .num{font-variant-numeric:tabular-nums}
  .wrap{max-width:1000px;margin:0 auto;padding:32px 22px 72px}
  h1{font-size:16px;margin:0;letter-spacing:.3px;font-weight:700;display:flex;align-items:center}
  h1::before{content:"";width:11px;height:11px;border-radius:3px;margin-right:10px;
    background:linear-gradient(135deg,var(--accent),#9b6bff)}
  .sub{color:var(--mute);font-size:12px;margin:5px 0 24px;padding-left:21px}
  section{background:var(--panel);border:1px solid var(--line);border-radius:16px;
    padding:18px 20px;margin-bottom:14px;box-shadow:0 1px 0 rgba(255,255,255,.02) inset,0 2px 10px rgba(0,0,0,.15)}
  h2{font-size:11px;text-transform:uppercase;letter-spacing:.09em;color:var(--dim);
    margin:0 0 15px;font-weight:600}

  /* hero */
  .hero{display:flex;gap:20px;flex-wrap:wrap;align-items:stretch}
  .hero .main{flex:1 1 220px;display:flex;flex-direction:column;justify-content:center;padding:2px 0}
  .hero .main .k{color:var(--dim);font-size:11px;text-transform:uppercase;letter-spacing:.09em}
  .hero .main .big{font-size:44px;font-weight:770;letter-spacing:-2px;line-height:1.02;margin-top:6px;
    font-variant-numeric:tabular-nums}
  .hero .main .kzt{color:var(--watch);font-size:15px;font-weight:600;margin-top:3px;font-variant-numeric:tabular-nums}
  .hero .main .meta{color:var(--mute);font-size:12px;margin-top:10px;line-height:1.4}
  .hero .side{flex:2 1 380px;display:grid;grid-template-columns:repeat(2,1fr);gap:11px}
  .mini{background:var(--panel2);border:1px solid var(--line);border-radius:12px;padding:13px 15px;
    transition:border-color .16s ease,transform .16s ease}
  .mini:hover{border-color:var(--line2);transform:translateY(-1px)}
  .mini .k{color:var(--dim);font-size:10px;text-transform:uppercase;letter-spacing:.06em}
  .mini .v{font-size:21px;font-weight:700;margin-top:5px;font-variant-numeric:tabular-nums}
  .mini .t{color:var(--mute);font-size:11px;margin-top:3px;font-variant-numeric:tabular-nums}
  .mini.good .v{color:var(--good)}

  /* meters */
  .meter{margin:15px 0}
  .meter:first-of-type{margin-top:2px}
  .meter .top{display:flex;justify-content:space-between;align-items:baseline;margin-bottom:7px}
  .meter .name{color:var(--dim);font-size:12px;text-transform:uppercase;letter-spacing:.05em;
    display:flex;align-items:center}
  .meter .val{font-size:18px;font-weight:700;font-variant-numeric:tabular-nums}
  .verdict{font-size:10px;padding:2px 8px;border-radius:20px;margin-left:9px;font-weight:700;
    text-transform:uppercase;letter-spacing:.04em}
  .verdict.good{background:rgba(53,201,138,.15);color:#5fe0a8}
  .verdict.watch{background:rgba(242,179,77,.15);color:#f7c877}
  .verdict.serious{background:rgba(242,114,77,.16);color:#ff9b7d}
  .mtrack{height:10px;background:#0b0e13;border-radius:6px;overflow:hidden;box-shadow:0 0 0 1px rgba(0,0,0,.3) inset}
  .mfill{height:100%;border-radius:6px;transition:width .55s cubic-bezier(.4,0,.2,1)}
  .mhint{color:var(--mute);font-size:11px;margin-top:6px}

  /* columns (time) */
  .chart{display:flex;align-items:flex-end;gap:14px;height:152px;padding-top:20px;border-bottom:1px solid var(--line)}
  .col{flex:1;display:flex;flex-direction:column;align-items:center;justify-content:flex-end;height:100%}
  .col .cv{font-size:11px;color:var(--dim);margin-bottom:6px;font-variant-numeric:tabular-nums}
  .col .cbar{width:100%;max-width:56px;min-height:3px;border-radius:5px 5px 0 0;
    background:linear-gradient(180deg,#7aabff,#4f88e6);transition:filter .16s ease}
  .col:hover .cbar{filter:brightness(1.14)}
  .col .cd{font-size:11px;color:var(--mute);margin-top:9px;font-variant-numeric:tabular-nums}

  /* bar lists */
  .cols{display:grid;grid-template-columns:1fr 1fr;gap:16px}
  @media(max-width:680px){.cols{grid-template-columns:1fr}.hero .side{grid-template-columns:1fr}}
  .brow{margin:12px 0}
  .brow .bt{display:flex;justify-content:space-between;font-size:12px;margin-bottom:6px;gap:8px}
  .brow .bn{color:var(--dim);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .brow .bv{color:var(--txt);font-variant-numeric:tabular-nums;flex:none}
  .btrack{height:8px;background:#0b0e13;border-radius:5px;overflow:hidden;box-shadow:0 0 0 1px rgba(0,0,0,.3) inset}
  .bfill{height:100%;background:linear-gradient(90deg,#4f88e6,#7aabff);border-radius:5px;transition:filter .16s ease}
  .brow:hover .bfill{filter:brightness(1.13)}

  /* token mix */
  .mix{display:flex;height:26px;border-radius:8px;overflow:hidden;margin-bottom:13px;background:var(--panel)}
  .mix span{display:block;border-right:2px solid var(--panel);transition:filter .16s ease}
  .mix span:last-child{border-right:0}
  .mix:hover span{filter:saturate(1.1)}
  .legend{display:flex;gap:18px;flex-wrap:wrap;color:var(--dim);font-size:12px}
  .legend span{font-variant-numeric:tabular-nums}
  .dot{display:inline-block;width:9px;height:9px;border-radius:3px;margin-right:7px;vertical-align:0}

  /* insights */
  .icard{display:flex;gap:11px;padding:12px 14px;border-radius:12px;background:var(--panel2);
    border-left:3px solid var(--line);margin:9px 0;transition:background .16s ease,transform .16s ease}
  .icard:hover{background:#181c24;transform:translateX(1px)}
  .icard.good{border-left-color:var(--good)}
  .icard.watch{border-left-color:var(--watch)}
  .icard.info{border-left-color:var(--accent)}
  .icard .ic{font-size:13px;line-height:1.55;flex:none;width:14px;text-align:center}
  .icard.good .ic{color:var(--good)} .icard.watch .ic{color:var(--watch)} .icard.info .ic{color:var(--accent)}
  .icard .tx{font-size:13px;color:var(--txt)}

  /* table */
  table{width:100%;border-collapse:collapse}
  td,th{padding:8px;text-align:left;border-bottom:1px solid var(--line)}
  tr:last-child td{border-bottom:0}
  tr:hover td{background:rgba(255,255,255,.025)}
  th{color:var(--dim);font-size:11px;text-transform:uppercase;letter-spacing:.05em;font-weight:600}
  td.amt{text-align:right;white-space:nowrap;font-variant-numeric:tabular-nums;font-weight:600}
  code{color:var(--mute);font-size:12px}
</style>
</head>
<body>
<div class="wrap">
  <h1>cc-econ</h1>
  <div class="sub" id="sub">Claude Code token economics</div>
  <div id="app"></div>
</div>
<script>
var D = __DATA__;
var C={good:"#35c98a",watch:"#f2b34d",serious:"#f2724d"};

function money(x){return "$"+x.toLocaleString("en-US",{minimumFractionDigits:2,maximumFractionDigits:2});}
function money0(x){return "$"+Math.round(x).toLocaleString("en-US");}
function commas(n){return n.toLocaleString("en-US");}
function kzt(usd){return D.kzt>0 ? "₸"+Math.round(usd*D.kzt).toLocaleString("en-US") : "";}
function el(tag,cls,html){var e=document.createElement(tag);if(cls)e.className=cls;if(html!=null)e.innerHTML=html;return e;}

document.getElementById("sub").textContent =
  "shadow price (Max plan) — API-list-equivalent, not a bill · USD→KZT ₸"+(D.kzt||0).toFixed(1)+" · reload to refresh";

var app=document.getElementById("app");

/* ---- hero ---- */
(function(){
  var s=el("section","hero");
  var main=el("div","main");
  main.appendChild(el("div","k","Total shadow spend"));
  main.appendChild(el("div","big",money0(D.total)));
  var kz=kzt(D.total); if(kz)main.appendChild(el("div","kzt",kz));
  main.appendChild(el("div","meta",commas(D.turns)+" turns · list-price equivalent of your usage"));
  s.appendChild(main);
  var side=el("div","side");
  function mini(k,v,t,cls){var m=el("div","mini"+(cls?" "+cls:""));m.appendChild(el("div","k",k));
    m.appendChild(el("div","v",v));if(t)m.appendChild(el("div","t",t));return m;}
  side.appendChild(mini("This month",money0(D.month),kzt(D.month)));
  side.appendChild(mini("Today",money0(D.today),kzt(D.today)));
  side.appendChild(mini("Proj. month",money0(D.monthProjected),"~"+money0(D.yearRate)+"/yr"));
  side.appendChild(mini("Cache saved",money0(D.cacheSavings),kzt(D.cacheSavings),"good"));
  s.appendChild(side);
  app.appendChild(s);
})();

/* ---- efficiency meters ---- */
(function(){
  var s=el("section"); s.appendChild(el("h2",null,"Efficiency"));
  function status(kind,v){
    if(kind==="hit")   return v>=85?["good","healthy"]:(v>=60?["watch","slack"]:["serious","breaking"]);
    if(kind==="churn") return v<=15?["good","lean"]:(v<=30?["watch","high"]:["serious","heavy"]);
    return v<=30?["good","lean"]:["watch","high"]; // output
  }
  function meter(name,pctv,kind,hint){
    var st=status(kind,pctv);
    var m=el("div","meter");
    var top=el("div","top");
    top.appendChild(el("div","name",name+'<span class="verdict '+st[0]+'">'+st[1]+"</span>"));
    top.appendChild(el("div","val",pctv.toFixed(0)+"%"));
    m.appendChild(top);
    var tr=el("div","mtrack");
    var f=el("div","mfill"); f.style.width=Math.min(100,pctv)+"%"; f.style.background=C[st[0]];
    tr.appendChild(f); m.appendChild(tr);
    m.appendChild(el("div","mhint",hint));
    return m;
  }
  s.appendChild(meter("Cache hit rate",D.cacheHitRate*100,"hit","share of context served warm from cache (higher = better)"));
  s.appendChild(meter("Cache churn",D.cacheChurnPct,"churn","spend going to cache-writes — your prefix/plugin overhead (lower = better)"));
  s.appendChild(meter("Output share",D.outputSharePct,"output","spend on generated tokens, billed 5x input (lower = leaner)"));
  app.appendChild(s);
})();

/* ---- daily spend as columns ---- */
(function(){
  var s=el("section"); s.appendChild(el("h2",null,"Daily spend"));
  var max=0; D.byDay.forEach(function(d){if(d.cost>max)max=d.cost;});
  var chart=el("div","chart");
  D.byDay.forEach(function(d){
    var c=el("div","col");
    c.appendChild(el("div","cv",money0(d.cost)));
    var bar=el("div","cbar"); bar.style.height=(max>0?Math.max(3,d.cost/max*110):3)+"px";
    bar.title=d.name+": "+money(d.cost);
    c.appendChild(bar);
    c.appendChild(el("div","cd",d.name.slice(5))); // MM-DD
    chart.appendChild(c);
  });
  s.appendChild(chart); app.appendChild(s);
})();

/* ---- by project + by model ---- */
(function(){
  function barlist(title,rows){
    var s=el("section"); s.appendChild(el("h2",null,title));
    var max=0; rows.forEach(function(r){if(r.cost>max)max=r.cost;});
    rows.forEach(function(r){
      var row=el("div","brow");
      var t=el("div","bt");
      t.appendChild(el("div","bn",r.name));
      t.appendChild(el("div","bv",money0(r.cost)));
      row.appendChild(t);
      var tr=el("div","btrack"); var f=el("div","bfill");
      f.style.width=(max>0?Math.max(2,r.cost/max*100):0)+"%"; tr.appendChild(f);
      row.appendChild(tr); s.appendChild(row);
    });
    return s;
  }
  var cols=el("div","cols");
  cols.appendChild(barlist("By project",D.byProject));
  cols.appendChild(barlist("By model",D.byModel));
  app.appendChild(cols);
})();

/* ---- token mix ---- */
(function(){
  var s=el("section"); s.appendChild(el("h2",null,"Token mix"));
  var parts=[
    {k:"input",v:D.in,c:"#5b9dff"},
    {k:"output",v:D.out,c:"#a97bff"},
    {k:"cache-write",v:D.cw,c:"#f2b34d"},
    {k:"cache-read",v:D.cr,c:"#35c98a"}
  ];
  var tot=parts.reduce(function(a,p){return a+p.v;},0)||1;
  var mix=el("div","mix");
  parts.forEach(function(p){var sp=el("span");sp.style.width=(p.v/tot*100)+"%";sp.style.background=p.c;
    sp.title=p.k+": "+commas(p.v);mix.appendChild(sp);});
  s.appendChild(mix);
  var lg=el("div","legend");
  parts.forEach(function(p){lg.appendChild(el("span",null,
    '<span class="dot" style="background:'+p.c+'"></span>'+p.k+" · "+commas(p.v)));});
  s.appendChild(lg); app.appendChild(s);
})();

/* ---- insights as cards ---- */
(function(){
  var s=el("section"); s.appendChild(el("h2",null,"Insights & optimization"));
  var icon={good:"✓",watch:"▲",info:"•"};
  D.insights.forEach(function(x){
    var c=el("div","icard "+x.level);
    c.appendChild(el("div","ic",icon[x.level]||"•"));
    c.appendChild(el("div","tx",x.text));
    s.appendChild(c);
  });
  app.appendChild(s);
})();

/* ---- top sessions ---- */
(function(){
  var s=el("section"); s.appendChild(el("h2",null,"Top costly sessions"));
  var t=el("table");
  t.innerHTML="<tr><th>cost</th><th>project</th><th>session</th></tr>";
  D.topSessions.forEach(function(x){
    var tr=el("tr");
    tr.innerHTML="<td class='amt'>"+money0(x.cost)+"</td><td>"+x.project+
      "</td><td><code>"+x.session.slice(0,8)+"</code></td>";
    t.appendChild(tr);
  });
  s.appendChild(t); app.appendChild(s);
})();
</script>
</body>
</html>`
