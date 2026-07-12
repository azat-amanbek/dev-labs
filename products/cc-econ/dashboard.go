package main

// dashboardHTML is a self-contained page (no external CDN). The server injects
// the report JSON in place of __DATA__ on each request, so a reload re-reads
// the transcripts. Intentionally free of backtick template literals so it can
// live inside a Go raw-string literal.
const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>cc-econ — Claude Code economics</title>
<style>
  :root{
    --bg:#0f1115; --panel:#171a21; --line:#242833; --txt:#e6e8ee; --dim:#8b93a7;
    --accent:#6ea8fe; --good:#39d98a; --warn:#f0b24a;
  }
  *{box-sizing:border-box}
  body{margin:0;background:var(--bg);color:var(--txt);
    font:14px/1.5 -apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif}
  .wrap{max-width:980px;margin:0 auto;padding:28px 20px 60px}
  h1{font-size:18px;margin:0 0 2px} .sub{color:var(--dim);font-size:12px;margin-bottom:22px}
  .tiles{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin-bottom:26px}
  @media(max-width:680px){.tiles{grid-template-columns:repeat(2,1fr)}}
  .tile{background:var(--panel);border:1px solid var(--line);border-radius:12px;padding:14px 16px}
  .tile .k{color:var(--dim);font-size:11px;text-transform:uppercase;letter-spacing:.06em}
  .tile .v{font-size:24px;font-weight:650;margin-top:6px}
  .tile.good .v{color:var(--good)}
  .tile .n{color:var(--dim);font-size:11px;margin-top:4px}
  section{background:var(--panel);border:1px solid var(--line);border-radius:12px;
    padding:16px 18px;margin-bottom:16px}
  section h2{font-size:12px;text-transform:uppercase;letter-spacing:.06em;color:var(--dim);
    margin:0 0 14px;font-weight:600}
  .cols{display:grid;grid-template-columns:1fr 1fr;gap:16px}
  @media(max-width:680px){.cols{grid-template-columns:1fr}}
  .row{display:grid;grid-template-columns:130px 1fr 84px;align-items:center;gap:10px;margin:7px 0}
  .row .lbl{color:var(--dim);font-size:12px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .row .amt{text-align:right;font-variant-numeric:tabular-nums}
  .track{background:#10131a;border-radius:6px;height:16px;overflow:hidden}
  .fill{height:100%;border-radius:6px;background:var(--accent)}
  .fill.good{background:var(--good)}
  table{width:100%;border-collapse:collapse;font-variant-numeric:tabular-nums}
  td,th{padding:6px 8px;text-align:left;border-bottom:1px solid var(--line)}
  th{color:var(--dim);font-size:11px;text-transform:uppercase;letter-spacing:.05em;font-weight:600}
  td.amt{text-align:right}
  .mix{display:flex;height:22px;border-radius:6px;overflow:hidden;margin-bottom:10px}
  .mix span{display:block}
  .legend{display:flex;gap:16px;flex-wrap:wrap;color:var(--dim);font-size:12px}
  .dot{display:inline-block;width:10px;height:10px;border-radius:3px;margin-right:6px;vertical-align:-1px}
  code{color:var(--dim);font-size:12px}
</style>
</head>
<body>
<div class="wrap">
  <h1>cc-econ</h1>
  <div class="sub">Claude Code token economics · reload to refresh</div>
  <div id="app"></div>
</div>
<script>
var D = __DATA__;

function money(x){
  return "$" + x.toLocaleString("en-US",{minimumFractionDigits:2,maximumFractionDigits:2});
}
function commas(n){ return n.toLocaleString("en-US"); }
function el(tag,cls,html){
  var e=document.createElement(tag);
  if(cls)e.className=cls;
  if(html!=null)e.innerHTML=html;
  return e;
}
function tile(k,v,cls,note){
  var t=el("div","tile"+(cls?" "+cls:""));
  t.appendChild(el("div","k",k));
  t.appendChild(el("div","v",v));
  if(note)t.appendChild(el("div","n",note));
  return t;
}
function barRow(label,value,max,good){
  var r=el("div","row");
  r.appendChild(el("div","lbl",label));
  var track=el("div","track");
  var fill=el("div","fill"+(good?" good":""));
  var pct = max>0 ? Math.max(2, value/max*100) : 0;
  fill.style.width = pct+"%";
  track.appendChild(fill);
  r.appendChild(track);
  r.appendChild(el("div","amt",money(value)));
  return r;
}
function barSection(title,rows,good){
  var s=el("section");
  s.appendChild(el("h2",null,title));
  var max=0; rows.forEach(function(x){ if(x.cost>max)max=x.cost; });
  rows.forEach(function(x){ s.appendChild(barRow(x.name,x.cost,max,good)); });
  return s;
}

var app=document.getElementById("app");

// tiles
var tiles=el("div","tiles");
tiles.appendChild(tile("Total spend",money(D.total),null,commas(D.turns)+" assistant turns"));
tiles.appendChild(tile("This month",money(D.month)));
tiles.appendChild(tile("Today",money(D.today)));
tiles.appendChild(tile("Cache saved",money(D.cacheSavings),"good","vs full input rate"));
app.appendChild(tiles);

// daily spend
app.appendChild(barSection("Daily spend", D.byDay.map(function(d){
  return {name:d.name, cost:d.cost};
}), false));

// project + model side by side
var cols=el("div","cols");
cols.appendChild(barSection("By project", D.byProject, false));
cols.appendChild(barSection("By model", D.byModel, false));
app.appendChild(cols);

// token mix
(function(){
  var s=el("section");
  s.appendChild(el("h2",null,"Token mix"));
  var parts=[
    {k:"input",       v:D.in, c:"#6ea8fe"},
    {k:"output",      v:D.out,c:"#b98cff"},
    {k:"cache-write", v:D.cw, c:"#f0b24a"},
    {k:"cache-read",  v:D.cr, c:"#39d98a"}
  ];
  var tot=parts.reduce(function(a,p){return a+p.v;},0)||1;
  var mix=el("div","mix");
  parts.forEach(function(p){
    var span=el("span");
    span.style.width=(p.v/tot*100)+"%";
    span.style.background=p.c;
    span.title=p.k+": "+commas(p.v);
    mix.appendChild(span);
  });
  s.appendChild(mix);
  var lg=el("div","legend");
  parts.forEach(function(p){
    lg.appendChild(el("span",null,'<span class="dot" style="background:'+p.c+'"></span>'+
      p.k+" — "+commas(p.v)));
  });
  s.appendChild(lg);
  app.appendChild(s);
})();

// top sessions
(function(){
  var s=el("section");
  s.appendChild(el("h2",null,"Top costly sessions"));
  var t=el("table");
  t.innerHTML="<tr><th>cost</th><th>project</th><th>session</th></tr>";
  D.topSessions.forEach(function(x){
    var tr=el("tr");
    tr.innerHTML="<td class='amt'>"+money(x.cost)+"</td><td>"+x.project+
      "</td><td><code>"+x.session.slice(0,8)+"</code></td>";
    t.appendChild(tr);
  });
  s.appendChild(t);
  app.appendChild(s);
})();
</script>
</body>
</html>`
