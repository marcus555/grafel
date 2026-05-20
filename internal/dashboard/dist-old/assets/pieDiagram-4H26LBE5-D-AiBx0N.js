import{D as S,a$ as R,f as Q,W as Y,aO as tt,X as et,aP as at,a0 as rt,aR as nt,a as p,au as W,$ as it,r as st,aN as ot,aB as lt,B as ct,s as ut,P as pt}from"./MermaidBlock-BTms9C2y.js";import{p as dt}from"./chunk-4BX2VUAB-CvOfYe1S.js";import{p as gt}from"./wardley-L42UT6IY-cM4PfUF2.js";import{I as ft}from"./index-D7efKNdb.js";import{d as _}from"./arc-HCsYo3w7.js";import"./query-B1Ohp7i8.js";import"./vendor-CEiLhMIW.js";import"./radix-D6Aya7L5.js";function ht(t,a){return a<t?-1:a>t?1:a>=t?0:NaN}function mt(t){return t}function vt(){var t=mt,a=ht,f=null,y=S(0),s=S(R),d=S(0);function o(e){var n,l=(e=Q(e)).length,g,h,v=0,c=new Array(l),i=new Array(l),x=+y.apply(this,arguments),w=Math.min(R,Math.max(-R,s.apply(this,arguments)-x)),m,C=Math.min(Math.abs(w)/l,d.apply(this,arguments)),$=C*(w<0?-1:1),u;for(n=0;n<l;++n)(u=i[c[n]=n]=+t(e[n],n,e))>0&&(v+=u);for(a!=null?c.sort(function(A,D){return a(i[A],i[D])}):f!=null&&c.sort(function(A,D){return f(e[A],e[D])}),n=0,h=v?(w-l*$)/v:0;n<l;++n,x=m)g=c[n],u=i[g],m=x+(u>0?u*h:0)+$,i[g]={data:e[g],index:n,value:u,startAngle:x,endAngle:m,padAngle:C};return i}return o.value=function(e){return arguments.length?(t=typeof e=="function"?e:S(+e),o):t},o.sortValues=function(e){return arguments.length?(a=e,f=null,o):a},o.sort=function(e){return arguments.length?(f=e,a=null,o):f},o.startAngle=function(e){return arguments.length?(y=typeof e=="function"?e:S(+e),o):y},o.endAngle=function(e){return arguments.length?(s=typeof e=="function"?e:S(+e),o):s},o.padAngle=function(e){return arguments.length?(d=typeof e=="function"?e:S(+e),o):d},o}var xt=pt.pie,z={sections:new Map,showData:!1},T=z.sections,B=z.showData,St=structuredClone(xt),yt=p(()=>structuredClone(St),"getConfig"),wt=p(()=>{T=new Map,B=z.showData,ut()},"clear"),At=p(({label:t,value:a})=>{if(a<0)throw new Error(`"${t}" has invalid value: ${a}. Negative values are not allowed in pie charts. All slice values must be >= 0.`);T.has(t)||(T.set(t,a),W.debug(`added new section: ${t}, with value: ${a}`))},"addSection"),Dt=p(()=>T,"getSections"),Ct=p(t=>{B=t},"setShowData"),$t=p(()=>B,"getShowData"),V={getConfig:yt,clear:wt,setDiagramTitle:nt,getDiagramTitle:rt,setAccTitle:at,getAccTitle:et,setAccDescription:tt,getAccDescription:Y,addSection:At,getSections:Dt,setShowData:Ct,getShowData:$t},Tt=p((t,a)=>{dt(t,a),a.setShowData(t.showData),t.sections.map(a.addSection)},"populateDb"),kt={parse:p(async t=>{const a=await gt("pie",t);W.debug(a),Tt(a,V)},"parse")},Et=p(t=>`
  .pieCircle{
    stroke: ${t.pieStrokeColor};
    stroke-width : ${t.pieStrokeWidth};
    opacity : ${t.pieOpacity};
  }
  .pieOuterCircle{
    stroke: ${t.pieOuterStrokeColor};
    stroke-width: ${t.pieOuterStrokeWidth};
    fill: none;
  }
  .pieTitleText {
    text-anchor: middle;
    font-size: ${t.pieTitleTextSize};
    fill: ${t.pieTitleTextColor};
    font-family: ${t.fontFamily};
  }
  .slice {
    font-family: ${t.fontFamily};
    fill: ${t.pieSectionTextColor};
    font-size:${t.pieSectionTextSize};
    // fill: white;
  }
  .legend text {
    fill: ${t.pieLegendTextColor};
    font-family: ${t.fontFamily};
    font-size: ${t.pieLegendTextSize};
  }
`,"getStyles"),Mt=Et,bt=p(t=>{const a=[...t.values()].reduce((s,d)=>s+d,0),f=[...t.entries()].map(([s,d])=>({label:s,value:d})).filter(s=>s.value/a*100>=1);return vt().value(s=>s.value).sort(null)(f)},"createPieArcs"),Rt=p((t,a,f,y)=>{var O;W.debug(`rendering pie chart
`+t);const s=y.db,d=it(),o=st(s.getConfig(),d.pie),e=40,n=18,l=4,g=450,h=g,v=ot(a),c=v.append("g");c.attr("transform","translate("+h/2+","+g/2+")");const{themeVariables:i}=d;let[x]=lt(i.pieOuterStrokeWidth);x??(x=2);const w=o.textPosition,m=Math.min(h,g)/2-e,C=_().innerRadius(0).outerRadius(m),$=_().innerRadius(m*w).outerRadius(m*w);c.append("circle").attr("cx",0).attr("cy",0).attr("r",m+x/2).attr("class","pieOuterCircle");const u=s.getSections(),A=bt(u),D=[i.pie1,i.pie2,i.pie3,i.pie4,i.pie5,i.pie6,i.pie7,i.pie8,i.pie9,i.pie10,i.pie11,i.pie12];let k=0;u.forEach(r=>{k+=r});const F=A.filter(r=>(r.data.value/k*100).toFixed(0)!=="0"),E=ft(D).domain([...u.keys()]);c.selectAll("mySlices").data(F).enter().append("path").attr("d",C).attr("fill",r=>E(r.data.label)).attr("class","pieCircle"),c.selectAll("mySlices").data(F).enter().append("text").text(r=>(r.data.value/k*100).toFixed(0)+"%").attr("transform",r=>"translate("+$.centroid(r)+")").style("text-anchor","middle").attr("class","slice");const U=c.append("text").text(s.getDiagramTitle()).attr("x",0).attr("y",-400/2).attr("class","pieTitleText"),N=[...u.entries()].map(([r,b])=>({label:r,value:b})),M=c.selectAll(".legend").data(N).enter().append("g").attr("class","legend").attr("transform",(r,b)=>{const I=n+l,H=I*N.length/2,J=12*n,K=b*I-H;return"translate("+J+","+K+")"});M.append("rect").attr("width",n).attr("height",n).style("fill",r=>E(r.label)).style("stroke",r=>E(r.label)),M.append("text").attr("x",n+l).attr("y",n-l).text(r=>s.getShowData()?`${r.label} [${r.value}]`:r.label);const X=Math.max(...M.selectAll("text").nodes().map(r=>(r==null?void 0:r.getBoundingClientRect().width)??0)),j=h+e+n+l+X,P=((O=U.node())==null?void 0:O.getBoundingClientRect().width)??0,Z=h/2-P/2,q=h/2+P/2,G=Math.min(0,Z),L=Math.max(j,q)-G;v.attr("viewBox",`${G} 0 ${L} ${g}`),ct(v,g,L,o.useMaxWidth)},"draw"),Wt={draw:Rt},_t={parser:kt,db:V,renderer:Wt,styles:Mt};export{_t as diagram};
//# sourceMappingURL=pieDiagram-4H26LBE5-D-AiBx0N.js.map
