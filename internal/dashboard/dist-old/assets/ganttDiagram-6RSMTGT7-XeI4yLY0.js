import{W as Ae,aO as Oe,a0 as We,aR as Ne,X as Pe,aP as Re,a as c,$ as ft,B as Ve,L as G,au as ot,x as ze,Q as He,s as Be,b6 as qe}from"./MermaidBlock-BTms9C2y.js";import{f as Yt}from"./vendor-CEiLhMIW.js";import{k as Xe,e as ye,M as Ge,K as je,j as ge,l as pe,a as ve,H as Ft,s as Ue,O as bt,Q as Ze,F as Qe,A as Ke,x as Je,h as ts,V as se,$ as ie,a1 as es,a0 as ss,W as is,a2 as rs,a4 as ns,a3 as as,Z as os,U as re,X as ne,Y as ae,N as oe,E as ce,i as cs}from"./index-D7efKNdb.js";import"./query-B1Ohp7i8.js";import"./radix-D6Aya7L5.js";const Et=18,xe=.96422,Te=1,be=.82521,we=4/29,ht=6/29,_e=3*ht*ht,ls=ht*ht*ht;function De(t){if(t instanceof et)return new et(t.l,t.a,t.b,t.opacity);if(t instanceof it)return Se(t);t instanceof ye||(t=Ge(t));var s=Nt(t.r),r=Nt(t.g),i=Nt(t.b),a=At((.2225045*s+.7168786*r+.0606169*i)/Te),k,y;return s===r&&r===i?k=y=a:(k=At((.4360747*s+.3850649*r+.1430804*i)/xe),y=At((.0139322*s+.0971045*r+.7141733*i)/be)),new et(116*a-16,500*(k-a),200*(a-y),t.opacity)}function us(t,s,r,i){return arguments.length===1?De(t):new et(t,s,r,i??1)}function et(t,s,r,i){this.l=+t,this.a=+s,this.b=+r,this.opacity=+i}ge(et,us,pe(ve,{brighter(t){return new et(this.l+Et*(t??1),this.a,this.b,this.opacity)},darker(t){return new et(this.l-Et*(t??1),this.a,this.b,this.opacity)},rgb(){var t=(this.l+16)/116,s=isNaN(this.a)?t:t+this.a/500,r=isNaN(this.b)?t:t-this.b/200;return s=xe*Ot(s),t=Te*Ot(t),r=be*Ot(r),new ye(Wt(3.1338561*s-1.6168667*t-.4906146*r),Wt(-.9787684*s+1.9161415*t+.033454*r),Wt(.0719453*s-.2289914*t+1.4052427*r),this.opacity)}}));function At(t){return t>ls?Math.pow(t,1/3):t/_e+we}function Ot(t){return t>ht?t*t*t:_e*(t-we)}function Wt(t){return 255*(t<=.0031308?12.92*t:1.055*Math.pow(t,1/2.4)-.055)}function Nt(t){return(t/=255)<=.04045?t/12.92:Math.pow((t+.055)/1.055,2.4)}function ds(t){if(t instanceof it)return new it(t.h,t.c,t.l,t.opacity);if(t instanceof et||(t=De(t)),t.a===0&&t.b===0)return new it(NaN,0<t.l&&t.l<100?0:NaN,t.l,t.opacity);var s=Math.atan2(t.b,t.a)*Xe;return new it(s<0?s+360:s,Math.sqrt(t.a*t.a+t.b*t.b),t.l,t.opacity)}function Rt(t,s,r,i){return arguments.length===1?ds(t):new it(t,s,r,i??1)}function it(t,s,r,i){this.h=+t,this.c=+s,this.l=+r,this.opacity=+i}function Se(t){if(isNaN(t.h))return new et(t.l,0,0,t.opacity);var s=t.h*je;return new et(t.l,Math.cos(s)*t.c,Math.sin(s)*t.c,t.opacity)}ge(it,Rt,pe(ve,{brighter(t){return new it(this.h,this.c,this.l+Et*(t??1),this.opacity)},darker(t){return new it(this.h,this.c,this.l-Et*(t??1),this.opacity)},rgb(){return Se(this).rgb()}}));function fs(t){return function(s,r){var i=t((s=Rt(s)).h,(r=Rt(r)).h),a=Ft(s.c,r.c),k=Ft(s.l,r.l),y=Ft(s.opacity,r.opacity);return function(x){return s.h=i(x),s.c=a(x),s.l=k(x),s.opacity=y(x),s+""}}}const hs=fs(Ue);var wt={exports:{}},ms=wt.exports,le;function ks(){return le||(le=1,(function(t,s){(function(r,i){t.exports=i()})(ms,(function(){var r="day";return function(i,a,k){var y=function(F){return F.add(4-F.isoWeekday(),r)},x=a.prototype;x.isoWeekYear=function(){return y(this).year()},x.isoWeek=function(F){if(!this.$utils().u(F))return this.add(7*(F-this.isoWeek()),r);var w,A,P,R,V=y(this),M=(w=this.isoWeekYear(),A=this.$u,P=(A?k.utc:k)().year(w).startOf("year"),R=4-P.isoWeekday(),P.isoWeekday()>4&&(R+=7),P.add(R,r));return V.diff(M,"week")+1},x.isoWeekday=function(F){return this.$utils().u(F)?this.day()||7:this.day(this.day()%7?F:F-7)};var O=x.startOf;x.startOf=function(F,w){var A=this.$utils(),P=!!A.u(w)||w;return A.p(F)==="isoweek"?P?this.date(this.date()-(this.isoWeekday()-1)).startOf("day"):this.date(this.date()-1-(this.isoWeekday()-1)+7).endOf("day"):O.bind(this)(F,w)}}}))})(wt)),wt.exports}var ys=ks();const gs=Yt(ys);var _t={exports:{}},ps=_t.exports,ue;function vs(){return ue||(ue=1,(function(t,s){(function(r,i){t.exports=i()})(ps,(function(){var r={LTS:"h:mm:ss A",LT:"h:mm A",L:"MM/DD/YYYY",LL:"MMMM D, YYYY",LLL:"MMMM D, YYYY h:mm A",LLLL:"dddd, MMMM D, YYYY h:mm A"},i=/(\[[^[]*\])|([-_:/.,()\s]+)|(A|a|Q|YYYY|YY?|ww?|MM?M?M?|Do|DD?|hh?|HH?|mm?|ss?|S{1,3}|z|ZZ?)/g,a=/\d/,k=/\d\d/,y=/\d\d?/,x=/\d*[^-_:/,()\s\d]+/,O={},F=function(D){return(D=+D)+(D>68?1900:2e3)},w=function(D){return function(S){this[D]=+S}},A=[/[+-]\d\d:?(\d\d)?|Z/,function(D){(this.zone||(this.zone={})).offset=(function(S){if(!S||S==="Z")return 0;var W=S.match(/([+-]|\d\d)/g),Y=60*W[1]+(+W[2]||0);return Y===0?0:W[0]==="+"?-Y:Y})(D)}],P=function(D){var S=O[D];return S&&(S.indexOf?S:S.s.concat(S.f))},R=function(D,S){var W,Y=O.meridiem;if(Y){for(var z=1;z<=24;z+=1)if(D.indexOf(Y(z,0,S))>-1){W=z>12;break}}else W=D===(S?"pm":"PM");return W},V={A:[x,function(D){this.afternoon=R(D,!1)}],a:[x,function(D){this.afternoon=R(D,!0)}],Q:[a,function(D){this.month=3*(D-1)+1}],S:[a,function(D){this.milliseconds=100*+D}],SS:[k,function(D){this.milliseconds=10*+D}],SSS:[/\d{3}/,function(D){this.milliseconds=+D}],s:[y,w("seconds")],ss:[y,w("seconds")],m:[y,w("minutes")],mm:[y,w("minutes")],H:[y,w("hours")],h:[y,w("hours")],HH:[y,w("hours")],hh:[y,w("hours")],D:[y,w("day")],DD:[k,w("day")],Do:[x,function(D){var S=O.ordinal,W=D.match(/\d+/);if(this.day=W[0],S)for(var Y=1;Y<=31;Y+=1)S(Y).replace(/\[|\]/g,"")===D&&(this.day=Y)}],w:[y,w("week")],ww:[k,w("week")],M:[y,w("month")],MM:[k,w("month")],MMM:[x,function(D){var S=P("months"),W=(P("monthsShort")||S.map((function(Y){return Y.slice(0,3)}))).indexOf(D)+1;if(W<1)throw new Error;this.month=W%12||W}],MMMM:[x,function(D){var S=P("months").indexOf(D)+1;if(S<1)throw new Error;this.month=S%12||S}],Y:[/[+-]?\d+/,w("year")],YY:[k,function(D){this.year=F(D)}],YYYY:[/\d{4}/,w("year")],Z:A,ZZ:A};function M(D){var S,W;S=D,W=O&&O.formats;for(var Y=(D=S.replace(/(\[[^\]]+])|(LTS?|l{1,4}|L{1,4})/g,(function(v,p,g){var f=g&&g.toUpperCase();return p||W[g]||r[g]||W[f].replace(/(\[[^\]]+])|(MMMM|MM|DD|dddd)/g,(function(o,l,h){return l||h.slice(1)}))}))).match(i),z=Y.length,q=0;q<z;q+=1){var E=Y[q],T=V[E],d=T&&T[0],u=T&&T[1];Y[q]=u?{regex:d,parser:u}:E.replace(/^\[|\]$/g,"")}return function(v){for(var p={},g=0,f=0;g<z;g+=1){var o=Y[g];if(typeof o=="string")f+=o.length;else{var l=o.regex,h=o.parser,m=v.slice(f),b=l.exec(m)[0];h.call(p,b),v=v.replace(b,"")}}return(function(n){var N=n.afternoon;if(N!==void 0){var e=n.hours;N?e<12&&(n.hours+=12):e===12&&(n.hours=0),delete n.afternoon}})(p),p}}return function(D,S,W){W.p.customParseFormat=!0,D&&D.parseTwoDigitYear&&(F=D.parseTwoDigitYear);var Y=S.prototype,z=Y.parse;Y.parse=function(q){var E=q.date,T=q.utc,d=q.args;this.$u=T;var u=d[1];if(typeof u=="string"){var v=d[2]===!0,p=d[3]===!0,g=v||p,f=d[2];p&&(f=d[2]),O=this.$locale(),!v&&f&&(O=W.Ls[f]),this.$d=(function(m,b,n,N){try{if(["x","X"].indexOf(b)>-1)return new Date((b==="X"?1e3:1)*m);var e=M(b)(m),_=e.year,L=e.month,$=e.day,I=e.hours,X=e.minutes,C=e.seconds,Q=e.milliseconds,rt=e.zone,lt=e.week,yt=new Date,gt=$||(_||L?1:yt.getDate()),ut=_||yt.getFullYear(),H=0;_&&!L||(H=L>0?L-1:yt.getMonth());var Z,j=I||0,at=X||0,K=C||0,nt=Q||0;return rt?new Date(Date.UTC(ut,H,gt,j,at,K,nt+60*rt.offset*1e3)):n?new Date(Date.UTC(ut,H,gt,j,at,K,nt)):(Z=new Date(ut,H,gt,j,at,K,nt),lt&&(Z=N(Z).week(lt).toDate()),Z)}catch{return new Date("")}})(E,u,T,W),this.init(),f&&f!==!0&&(this.$L=this.locale(f).$L),g&&E!=this.format(u)&&(this.$d=new Date("")),O={}}else if(u instanceof Array)for(var o=u.length,l=1;l<=o;l+=1){d[1]=u[l-1];var h=W.apply(this,d);if(h.isValid()){this.$d=h.$d,this.$L=h.$L,this.init();break}l===o&&(this.$d=new Date(""))}else z.call(this,q)}}}))})(_t)),_t.exports}var xs=vs();const Ts=Yt(xs);var Dt={exports:{}},bs=Dt.exports,de;function ws(){return de||(de=1,(function(t,s){(function(r,i){t.exports=i()})(bs,(function(){return function(r,i){var a=i.prototype,k=a.format;a.format=function(y){var x=this,O=this.$locale();if(!this.isValid())return k.bind(this)(y);var F=this.$utils(),w=(y||"YYYY-MM-DDTHH:mm:ssZ").replace(/\[([^\]]+)]|Q|wo|ww|w|WW|W|zzz|z|gggg|GGGG|Do|X|x|k{1,2}|S/g,(function(A){switch(A){case"Q":return Math.ceil((x.$M+1)/3);case"Do":return O.ordinal(x.$D);case"gggg":return x.weekYear();case"GGGG":return x.isoWeekYear();case"wo":return O.ordinal(x.week(),"W");case"w":case"ww":return F.s(x.week(),A==="w"?1:2,"0");case"W":case"WW":return F.s(x.isoWeek(),A==="W"?1:2,"0");case"k":case"kk":return F.s(String(x.$H===0?24:x.$H),A==="k"?1:2,"0");case"X":return Math.floor(x.$d.getTime()/1e3);case"x":return x.$d.getTime();case"z":return"["+x.offsetName()+"]";case"zzz":return"["+x.offsetName("long")+"]";default:return A}}));return k.bind(this)(w)}}}))})(Dt)),Dt.exports}var _s=ws();const Ds=Yt(_s);var St={exports:{}},Ss=St.exports,fe;function Cs(){return fe||(fe=1,(function(t,s){(function(r,i){t.exports=i()})(Ss,(function(){var r,i,a=1e3,k=6e4,y=36e5,x=864e5,O=/\[([^\]]+)]|Y{1,4}|M{1,4}|D{1,2}|d{1,4}|H{1,2}|h{1,2}|a|A|m{1,2}|s{1,2}|Z{1,2}|SSS/g,F=31536e6,w=2628e6,A=/^(-|\+)?P(?:([-+]?[0-9,.]*)Y)?(?:([-+]?[0-9,.]*)M)?(?:([-+]?[0-9,.]*)W)?(?:([-+]?[0-9,.]*)D)?(?:T(?:([-+]?[0-9,.]*)H)?(?:([-+]?[0-9,.]*)M)?(?:([-+]?[0-9,.]*)S)?)?$/,P={years:F,months:w,days:x,hours:y,minutes:k,seconds:a,milliseconds:1,weeks:6048e5},R=function(E){return E instanceof z},V=function(E,T,d){return new z(E,d,T.$l)},M=function(E){return i.p(E)+"s"},D=function(E){return E<0},S=function(E){return D(E)?Math.ceil(E):Math.floor(E)},W=function(E){return Math.abs(E)},Y=function(E,T){return E?D(E)?{negative:!0,format:""+W(E)+T}:{negative:!1,format:""+E+T}:{negative:!1,format:""}},z=(function(){function E(d,u,v){var p=this;if(this.$d={},this.$l=v,d===void 0&&(this.$ms=0,this.parseFromMilliseconds()),u)return V(d*P[M(u)],this);if(typeof d=="number")return this.$ms=d,this.parseFromMilliseconds(),this;if(typeof d=="object")return Object.keys(d).forEach((function(o){p.$d[M(o)]=d[o]})),this.calMilliseconds(),this;if(typeof d=="string"){var g=d.match(A);if(g){var f=g.slice(2).map((function(o){return o!=null?Number(o):0}));return this.$d.years=f[0],this.$d.months=f[1],this.$d.weeks=f[2],this.$d.days=f[3],this.$d.hours=f[4],this.$d.minutes=f[5],this.$d.seconds=f[6],this.calMilliseconds(),this}}return this}var T=E.prototype;return T.calMilliseconds=function(){var d=this;this.$ms=Object.keys(this.$d).reduce((function(u,v){return u+(d.$d[v]||0)*P[v]}),0)},T.parseFromMilliseconds=function(){var d=this.$ms;this.$d.years=S(d/F),d%=F,this.$d.months=S(d/w),d%=w,this.$d.days=S(d/x),d%=x,this.$d.hours=S(d/y),d%=y,this.$d.minutes=S(d/k),d%=k,this.$d.seconds=S(d/a),d%=a,this.$d.milliseconds=d},T.toISOString=function(){var d=Y(this.$d.years,"Y"),u=Y(this.$d.months,"M"),v=+this.$d.days||0;this.$d.weeks&&(v+=7*this.$d.weeks);var p=Y(v,"D"),g=Y(this.$d.hours,"H"),f=Y(this.$d.minutes,"M"),o=this.$d.seconds||0;this.$d.milliseconds&&(o+=this.$d.milliseconds/1e3,o=Math.round(1e3*o)/1e3);var l=Y(o,"S"),h=d.negative||u.negative||p.negative||g.negative||f.negative||l.negative,m=g.format||f.format||l.format?"T":"",b=(h?"-":"")+"P"+d.format+u.format+p.format+m+g.format+f.format+l.format;return b==="P"||b==="-P"?"P0D":b},T.toJSON=function(){return this.toISOString()},T.format=function(d){var u=d||"YYYY-MM-DDTHH:mm:ss",v={Y:this.$d.years,YY:i.s(this.$d.years,2,"0"),YYYY:i.s(this.$d.years,4,"0"),M:this.$d.months,MM:i.s(this.$d.months,2,"0"),D:this.$d.days,DD:i.s(this.$d.days,2,"0"),H:this.$d.hours,HH:i.s(this.$d.hours,2,"0"),m:this.$d.minutes,mm:i.s(this.$d.minutes,2,"0"),s:this.$d.seconds,ss:i.s(this.$d.seconds,2,"0"),SSS:i.s(this.$d.milliseconds,3,"0")};return u.replace(O,(function(p,g){return g||String(v[p])}))},T.as=function(d){return this.$ms/P[M(d)]},T.get=function(d){var u=this.$ms,v=M(d);return v==="milliseconds"?u%=1e3:u=v==="weeks"?S(u/P[v]):this.$d[v],u||0},T.add=function(d,u,v){var p;return p=u?d*P[M(u)]:R(d)?d.$ms:V(d,this).$ms,V(this.$ms+p*(v?-1:1),this)},T.subtract=function(d,u){return this.add(d,u,!0)},T.locale=function(d){var u=this.clone();return u.$l=d,u},T.clone=function(){return V(this.$ms,this)},T.humanize=function(d){return r().add(this.$ms,"ms").locale(this.$l).fromNow(!d)},T.valueOf=function(){return this.asMilliseconds()},T.milliseconds=function(){return this.get("milliseconds")},T.asMilliseconds=function(){return this.as("milliseconds")},T.seconds=function(){return this.get("seconds")},T.asSeconds=function(){return this.as("seconds")},T.minutes=function(){return this.get("minutes")},T.asMinutes=function(){return this.as("minutes")},T.hours=function(){return this.get("hours")},T.asHours=function(){return this.as("hours")},T.days=function(){return this.get("days")},T.asDays=function(){return this.as("days")},T.weeks=function(){return this.get("weeks")},T.asWeeks=function(){return this.as("weeks")},T.months=function(){return this.get("months")},T.asMonths=function(){return this.as("months")},T.years=function(){return this.get("years")},T.asYears=function(){return this.as("years")},E})(),q=function(E,T,d){return E.add(T.years()*d,"y").add(T.months()*d,"M").add(T.days()*d,"d").add(T.hours()*d,"h").add(T.minutes()*d,"m").add(T.seconds()*d,"s").add(T.milliseconds()*d,"ms")};return function(E,T,d){r=d,i=d().$utils(),d.duration=function(p,g){var f=d.locale();return V(p,{$l:f},g)},d.isDuration=R;var u=T.prototype.add,v=T.prototype.subtract;T.prototype.add=function(p,g){return R(p)?q(this,p,1):u.bind(this)(p,g)},T.prototype.subtract=function(p,g){return R(p)?q(this,p,-1):v.bind(this)(p,g)}}}))})(St)),St.exports}var Ms=Cs();const Es=Yt(Ms);var Vt=(function(){var t=c(function(f,o,l,h){for(l=l||{},h=f.length;h--;l[f[h]]=o);return l},"o"),s=[6,8,10,12,13,14,15,16,17,18,20,21,22,23,24,25,26,27,28,29,30,31,33,35,36,38,40],r=[1,26],i=[1,27],a=[1,28],k=[1,29],y=[1,30],x=[1,31],O=[1,32],F=[1,33],w=[1,34],A=[1,9],P=[1,10],R=[1,11],V=[1,12],M=[1,13],D=[1,14],S=[1,15],W=[1,16],Y=[1,19],z=[1,20],q=[1,21],E=[1,22],T=[1,23],d=[1,25],u=[1,35],v={trace:c(function(){},"trace"),yy:{},symbols_:{error:2,start:3,gantt:4,document:5,EOF:6,line:7,SPACE:8,statement:9,NL:10,weekday:11,weekday_monday:12,weekday_tuesday:13,weekday_wednesday:14,weekday_thursday:15,weekday_friday:16,weekday_saturday:17,weekday_sunday:18,weekend:19,weekend_friday:20,weekend_saturday:21,dateFormat:22,inclusiveEndDates:23,topAxis:24,axisFormat:25,tickInterval:26,excludes:27,includes:28,todayMarker:29,title:30,acc_title:31,acc_title_value:32,acc_descr:33,acc_descr_value:34,acc_descr_multiline_value:35,section:36,clickStatement:37,taskTxt:38,taskData:39,click:40,callbackname:41,callbackargs:42,href:43,clickStatementDebug:44,$accept:0,$end:1},terminals_:{2:"error",4:"gantt",6:"EOF",8:"SPACE",10:"NL",12:"weekday_monday",13:"weekday_tuesday",14:"weekday_wednesday",15:"weekday_thursday",16:"weekday_friday",17:"weekday_saturday",18:"weekday_sunday",20:"weekend_friday",21:"weekend_saturday",22:"dateFormat",23:"inclusiveEndDates",24:"topAxis",25:"axisFormat",26:"tickInterval",27:"excludes",28:"includes",29:"todayMarker",30:"title",31:"acc_title",32:"acc_title_value",33:"acc_descr",34:"acc_descr_value",35:"acc_descr_multiline_value",36:"section",38:"taskTxt",39:"taskData",40:"click",41:"callbackname",42:"callbackargs",43:"href"},productions_:[0,[3,3],[5,0],[5,2],[7,2],[7,1],[7,1],[7,1],[11,1],[11,1],[11,1],[11,1],[11,1],[11,1],[11,1],[19,1],[19,1],[9,1],[9,1],[9,1],[9,1],[9,1],[9,1],[9,1],[9,1],[9,1],[9,1],[9,1],[9,2],[9,2],[9,1],[9,1],[9,1],[9,2],[37,2],[37,3],[37,3],[37,4],[37,3],[37,4],[37,2],[44,2],[44,3],[44,3],[44,4],[44,3],[44,4],[44,2]],performAction:c(function(o,l,h,m,b,n,N){var e=n.length-1;switch(b){case 1:return n[e-1];case 2:this.$=[];break;case 3:n[e-1].push(n[e]),this.$=n[e-1];break;case 4:case 5:this.$=n[e];break;case 6:case 7:this.$=[];break;case 8:m.setWeekday("monday");break;case 9:m.setWeekday("tuesday");break;case 10:m.setWeekday("wednesday");break;case 11:m.setWeekday("thursday");break;case 12:m.setWeekday("friday");break;case 13:m.setWeekday("saturday");break;case 14:m.setWeekday("sunday");break;case 15:m.setWeekend("friday");break;case 16:m.setWeekend("saturday");break;case 17:m.setDateFormat(n[e].substr(11)),this.$=n[e].substr(11);break;case 18:m.enableInclusiveEndDates(),this.$=n[e].substr(18);break;case 19:m.TopAxis(),this.$=n[e].substr(8);break;case 20:m.setAxisFormat(n[e].substr(11)),this.$=n[e].substr(11);break;case 21:m.setTickInterval(n[e].substr(13)),this.$=n[e].substr(13);break;case 22:m.setExcludes(n[e].substr(9)),this.$=n[e].substr(9);break;case 23:m.setIncludes(n[e].substr(9)),this.$=n[e].substr(9);break;case 24:m.setTodayMarker(n[e].substr(12)),this.$=n[e].substr(12);break;case 27:m.setDiagramTitle(n[e].substr(6)),this.$=n[e].substr(6);break;case 28:this.$=n[e].trim(),m.setAccTitle(this.$);break;case 29:case 30:this.$=n[e].trim(),m.setAccDescription(this.$);break;case 31:m.addSection(n[e].substr(8)),this.$=n[e].substr(8);break;case 33:m.addTask(n[e-1],n[e]),this.$="task";break;case 34:this.$=n[e-1],m.setClickEvent(n[e-1],n[e],null);break;case 35:this.$=n[e-2],m.setClickEvent(n[e-2],n[e-1],n[e]);break;case 36:this.$=n[e-2],m.setClickEvent(n[e-2],n[e-1],null),m.setLink(n[e-2],n[e]);break;case 37:this.$=n[e-3],m.setClickEvent(n[e-3],n[e-2],n[e-1]),m.setLink(n[e-3],n[e]);break;case 38:this.$=n[e-2],m.setClickEvent(n[e-2],n[e],null),m.setLink(n[e-2],n[e-1]);break;case 39:this.$=n[e-3],m.setClickEvent(n[e-3],n[e-1],n[e]),m.setLink(n[e-3],n[e-2]);break;case 40:this.$=n[e-1],m.setLink(n[e-1],n[e]);break;case 41:case 47:this.$=n[e-1]+" "+n[e];break;case 42:case 43:case 45:this.$=n[e-2]+" "+n[e-1]+" "+n[e];break;case 44:case 46:this.$=n[e-3]+" "+n[e-2]+" "+n[e-1]+" "+n[e];break}},"anonymous"),table:[{3:1,4:[1,2]},{1:[3]},t(s,[2,2],{5:3}),{6:[1,4],7:5,8:[1,6],9:7,10:[1,8],11:17,12:r,13:i,14:a,15:k,16:y,17:x,18:O,19:18,20:F,21:w,22:A,23:P,24:R,25:V,26:M,27:D,28:S,29:W,30:Y,31:z,33:q,35:E,36:T,37:24,38:d,40:u},t(s,[2,7],{1:[2,1]}),t(s,[2,3]),{9:36,11:17,12:r,13:i,14:a,15:k,16:y,17:x,18:O,19:18,20:F,21:w,22:A,23:P,24:R,25:V,26:M,27:D,28:S,29:W,30:Y,31:z,33:q,35:E,36:T,37:24,38:d,40:u},t(s,[2,5]),t(s,[2,6]),t(s,[2,17]),t(s,[2,18]),t(s,[2,19]),t(s,[2,20]),t(s,[2,21]),t(s,[2,22]),t(s,[2,23]),t(s,[2,24]),t(s,[2,25]),t(s,[2,26]),t(s,[2,27]),{32:[1,37]},{34:[1,38]},t(s,[2,30]),t(s,[2,31]),t(s,[2,32]),{39:[1,39]},t(s,[2,8]),t(s,[2,9]),t(s,[2,10]),t(s,[2,11]),t(s,[2,12]),t(s,[2,13]),t(s,[2,14]),t(s,[2,15]),t(s,[2,16]),{41:[1,40],43:[1,41]},t(s,[2,4]),t(s,[2,28]),t(s,[2,29]),t(s,[2,33]),t(s,[2,34],{42:[1,42],43:[1,43]}),t(s,[2,40],{41:[1,44]}),t(s,[2,35],{43:[1,45]}),t(s,[2,36]),t(s,[2,38],{42:[1,46]}),t(s,[2,37]),t(s,[2,39])],defaultActions:{},parseError:c(function(o,l){if(l.recoverable)this.trace(o);else{var h=new Error(o);throw h.hash=l,h}},"parseError"),parse:c(function(o){var l=this,h=[0],m=[],b=[null],n=[],N=this.table,e="",_=0,L=0,$=2,I=1,X=n.slice.call(arguments,1),C=Object.create(this.lexer),Q={yy:{}};for(var rt in this.yy)Object.prototype.hasOwnProperty.call(this.yy,rt)&&(Q.yy[rt]=this.yy[rt]);C.setInput(o,Q.yy),Q.yy.lexer=C,Q.yy.parser=this,typeof C.yylloc>"u"&&(C.yylloc={});var lt=C.yylloc;n.push(lt);var yt=C.options&&C.options.ranges;typeof Q.yy.parseError=="function"?this.parseError=Q.yy.parseError:this.parseError=Object.getPrototypeOf(this).parseError;function gt(U){h.length=h.length-2*U,b.length=b.length-U,n.length=n.length-U}c(gt,"popStack");function ut(){var U;return U=m.pop()||C.lex()||I,typeof U!="number"&&(U instanceof Array&&(m=U,U=m.pop()),U=l.symbols_[U]||U),U}c(ut,"lex");for(var H,Z,j,at,K={},nt,J,ee,Tt;;){if(Z=h[h.length-1],this.defaultActions[Z]?j=this.defaultActions[Z]:((H===null||typeof H>"u")&&(H=ut()),j=N[Z]&&N[Z][H]),typeof j>"u"||!j.length||!j[0]){var Lt="";Tt=[];for(nt in N[Z])this.terminals_[nt]&&nt>$&&Tt.push("'"+this.terminals_[nt]+"'");C.showPosition?Lt="Parse error on line "+(_+1)+`:
`+C.showPosition()+`
Expecting `+Tt.join(", ")+", got '"+(this.terminals_[H]||H)+"'":Lt="Parse error on line "+(_+1)+": Unexpected "+(H==I?"end of input":"'"+(this.terminals_[H]||H)+"'"),this.parseError(Lt,{text:C.match,token:this.terminals_[H]||H,line:C.yylineno,loc:lt,expected:Tt})}if(j[0]instanceof Array&&j.length>1)throw new Error("Parse Error: multiple actions possible at state: "+Z+", token: "+H);switch(j[0]){case 1:h.push(H),b.push(C.yytext),n.push(C.yylloc),h.push(j[1]),H=null,L=C.yyleng,e=C.yytext,_=C.yylineno,lt=C.yylloc;break;case 2:if(J=this.productions_[j[1]][1],K.$=b[b.length-J],K._$={first_line:n[n.length-(J||1)].first_line,last_line:n[n.length-1].last_line,first_column:n[n.length-(J||1)].first_column,last_column:n[n.length-1].last_column},yt&&(K._$.range=[n[n.length-(J||1)].range[0],n[n.length-1].range[1]]),at=this.performAction.apply(K,[e,L,_,Q.yy,j[1],b,n].concat(X)),typeof at<"u")return at;J&&(h=h.slice(0,-1*J*2),b=b.slice(0,-1*J),n=n.slice(0,-1*J)),h.push(this.productions_[j[1]][0]),b.push(K.$),n.push(K._$),ee=N[h[h.length-2]][h[h.length-1]],h.push(ee);break;case 3:return!0}}return!0},"parse")},p=(function(){var f={EOF:1,parseError:c(function(l,h){if(this.yy.parser)this.yy.parser.parseError(l,h);else throw new Error(l)},"parseError"),setInput:c(function(o,l){return this.yy=l||this.yy||{},this._input=o,this._more=this._backtrack=this.done=!1,this.yylineno=this.yyleng=0,this.yytext=this.matched=this.match="",this.conditionStack=["INITIAL"],this.yylloc={first_line:1,first_column:0,last_line:1,last_column:0},this.options.ranges&&(this.yylloc.range=[0,0]),this.offset=0,this},"setInput"),input:c(function(){var o=this._input[0];this.yytext+=o,this.yyleng++,this.offset++,this.match+=o,this.matched+=o;var l=o.match(/(?:\r\n?|\n).*/g);return l?(this.yylineno++,this.yylloc.last_line++):this.yylloc.last_column++,this.options.ranges&&this.yylloc.range[1]++,this._input=this._input.slice(1),o},"input"),unput:c(function(o){var l=o.length,h=o.split(/(?:\r\n?|\n)/g);this._input=o+this._input,this.yytext=this.yytext.substr(0,this.yytext.length-l),this.offset-=l;var m=this.match.split(/(?:\r\n?|\n)/g);this.match=this.match.substr(0,this.match.length-1),this.matched=this.matched.substr(0,this.matched.length-1),h.length-1&&(this.yylineno-=h.length-1);var b=this.yylloc.range;return this.yylloc={first_line:this.yylloc.first_line,last_line:this.yylineno+1,first_column:this.yylloc.first_column,last_column:h?(h.length===m.length?this.yylloc.first_column:0)+m[m.length-h.length].length-h[0].length:this.yylloc.first_column-l},this.options.ranges&&(this.yylloc.range=[b[0],b[0]+this.yyleng-l]),this.yyleng=this.yytext.length,this},"unput"),more:c(function(){return this._more=!0,this},"more"),reject:c(function(){if(this.options.backtrack_lexer)this._backtrack=!0;else return this.parseError("Lexical error on line "+(this.yylineno+1)+`. You can only invoke reject() in the lexer when the lexer is of the backtracking persuasion (options.backtrack_lexer = true).
`+this.showPosition(),{text:"",token:null,line:this.yylineno});return this},"reject"),less:c(function(o){this.unput(this.match.slice(o))},"less"),pastInput:c(function(){var o=this.matched.substr(0,this.matched.length-this.match.length);return(o.length>20?"...":"")+o.substr(-20).replace(/\n/g,"")},"pastInput"),upcomingInput:c(function(){var o=this.match;return o.length<20&&(o+=this._input.substr(0,20-o.length)),(o.substr(0,20)+(o.length>20?"...":"")).replace(/\n/g,"")},"upcomingInput"),showPosition:c(function(){var o=this.pastInput(),l=new Array(o.length+1).join("-");return o+this.upcomingInput()+`
`+l+"^"},"showPosition"),test_match:c(function(o,l){var h,m,b;if(this.options.backtrack_lexer&&(b={yylineno:this.yylineno,yylloc:{first_line:this.yylloc.first_line,last_line:this.last_line,first_column:this.yylloc.first_column,last_column:this.yylloc.last_column},yytext:this.yytext,match:this.match,matches:this.matches,matched:this.matched,yyleng:this.yyleng,offset:this.offset,_more:this._more,_input:this._input,yy:this.yy,conditionStack:this.conditionStack.slice(0),done:this.done},this.options.ranges&&(b.yylloc.range=this.yylloc.range.slice(0))),m=o[0].match(/(?:\r\n?|\n).*/g),m&&(this.yylineno+=m.length),this.yylloc={first_line:this.yylloc.last_line,last_line:this.yylineno+1,first_column:this.yylloc.last_column,last_column:m?m[m.length-1].length-m[m.length-1].match(/\r?\n?/)[0].length:this.yylloc.last_column+o[0].length},this.yytext+=o[0],this.match+=o[0],this.matches=o,this.yyleng=this.yytext.length,this.options.ranges&&(this.yylloc.range=[this.offset,this.offset+=this.yyleng]),this._more=!1,this._backtrack=!1,this._input=this._input.slice(o[0].length),this.matched+=o[0],h=this.performAction.call(this,this.yy,this,l,this.conditionStack[this.conditionStack.length-1]),this.done&&this._input&&(this.done=!1),h)return h;if(this._backtrack){for(var n in b)this[n]=b[n];return!1}return!1},"test_match"),next:c(function(){if(this.done)return this.EOF;this._input||(this.done=!0);var o,l,h,m;this._more||(this.yytext="",this.match="");for(var b=this._currentRules(),n=0;n<b.length;n++)if(h=this._input.match(this.rules[b[n]]),h&&(!l||h[0].length>l[0].length)){if(l=h,m=n,this.options.backtrack_lexer){if(o=this.test_match(h,b[n]),o!==!1)return o;if(this._backtrack){l=!1;continue}else return!1}else if(!this.options.flex)break}return l?(o=this.test_match(l,b[m]),o!==!1?o:!1):this._input===""?this.EOF:this.parseError("Lexical error on line "+(this.yylineno+1)+`. Unrecognized text.
`+this.showPosition(),{text:"",token:null,line:this.yylineno})},"next"),lex:c(function(){var l=this.next();return l||this.lex()},"lex"),begin:c(function(l){this.conditionStack.push(l)},"begin"),popState:c(function(){var l=this.conditionStack.length-1;return l>0?this.conditionStack.pop():this.conditionStack[0]},"popState"),_currentRules:c(function(){return this.conditionStack.length&&this.conditionStack[this.conditionStack.length-1]?this.conditions[this.conditionStack[this.conditionStack.length-1]].rules:this.conditions.INITIAL.rules},"_currentRules"),topState:c(function(l){return l=this.conditionStack.length-1-Math.abs(l||0),l>=0?this.conditionStack[l]:"INITIAL"},"topState"),pushState:c(function(l){this.begin(l)},"pushState"),stateStackSize:c(function(){return this.conditionStack.length},"stateStackSize"),options:{"case-insensitive":!0},performAction:c(function(l,h,m,b){switch(m){case 0:return this.begin("open_directive"),"open_directive";case 1:return this.begin("acc_title"),31;case 2:return this.popState(),"acc_title_value";case 3:return this.begin("acc_descr"),33;case 4:return this.popState(),"acc_descr_value";case 5:this.begin("acc_descr_multiline");break;case 6:this.popState();break;case 7:return"acc_descr_multiline_value";case 8:break;case 9:break;case 10:break;case 11:return 10;case 12:break;case 13:break;case 14:this.begin("href");break;case 15:this.popState();break;case 16:return 43;case 17:this.begin("callbackname");break;case 18:this.popState();break;case 19:this.popState(),this.begin("callbackargs");break;case 20:return 41;case 21:this.popState();break;case 22:return 42;case 23:this.begin("click");break;case 24:this.popState();break;case 25:return 40;case 26:return 4;case 27:return 22;case 28:return 23;case 29:return 24;case 30:return 25;case 31:return 26;case 32:return 28;case 33:return 27;case 34:return 29;case 35:return 12;case 36:return 13;case 37:return 14;case 38:return 15;case 39:return 16;case 40:return 17;case 41:return 18;case 42:return 20;case 43:return 21;case 44:return"date";case 45:return 30;case 46:return"accDescription";case 47:return 36;case 48:return 38;case 49:return 39;case 50:return":";case 51:return 6;case 52:return"INVALID"}},"anonymous"),rules:[/^(?:%%\{)/i,/^(?:accTitle\s*:\s*)/i,/^(?:(?!\n||)*[^\n]*)/i,/^(?:accDescr\s*:\s*)/i,/^(?:(?!\n||)*[^\n]*)/i,/^(?:accDescr\s*\{\s*)/i,/^(?:[\}])/i,/^(?:[^\}]*)/i,/^(?:%%(?!\{)*[^\n]*)/i,/^(?:[^\}]%%*[^\n]*)/i,/^(?:%%*[^\n]*[\n]*)/i,/^(?:[\n]+)/i,/^(?:\s+)/i,/^(?:%[^\n]*)/i,/^(?:href[\s]+["])/i,/^(?:["])/i,/^(?:[^"]*)/i,/^(?:call[\s]+)/i,/^(?:\([\s]*\))/i,/^(?:\()/i,/^(?:[^(]*)/i,/^(?:\))/i,/^(?:[^)]*)/i,/^(?:click[\s]+)/i,/^(?:[\s\n])/i,/^(?:[^\s\n]*)/i,/^(?:gantt\b)/i,/^(?:dateFormat\s[^#\n;]+)/i,/^(?:inclusiveEndDates\b)/i,/^(?:topAxis\b)/i,/^(?:axisFormat\s[^#\n;]+)/i,/^(?:tickInterval\s[^#\n;]+)/i,/^(?:includes\s[^#\n;]+)/i,/^(?:excludes\s[^#\n;]+)/i,/^(?:todayMarker\s[^\n;]+)/i,/^(?:weekday\s+monday\b)/i,/^(?:weekday\s+tuesday\b)/i,/^(?:weekday\s+wednesday\b)/i,/^(?:weekday\s+thursday\b)/i,/^(?:weekday\s+friday\b)/i,/^(?:weekday\s+saturday\b)/i,/^(?:weekday\s+sunday\b)/i,/^(?:weekend\s+friday\b)/i,/^(?:weekend\s+saturday\b)/i,/^(?:\d\d\d\d-\d\d-\d\d\b)/i,/^(?:title\s[^\n]+)/i,/^(?:accDescription\s[^#\n;]+)/i,/^(?:section\s[^\n]+)/i,/^(?:[^:\n]+)/i,/^(?::[^#\n;]+)/i,/^(?::)/i,/^(?:$)/i,/^(?:.)/i],conditions:{acc_descr_multiline:{rules:[6,7],inclusive:!1},acc_descr:{rules:[4],inclusive:!1},acc_title:{rules:[2],inclusive:!1},callbackargs:{rules:[21,22],inclusive:!1},callbackname:{rules:[18,19,20],inclusive:!1},href:{rules:[15,16],inclusive:!1},click:{rules:[24,25],inclusive:!1},INITIAL:{rules:[0,1,3,5,8,9,10,11,12,13,14,17,23,26,27,28,29,30,31,32,33,34,35,36,37,38,39,40,41,42,43,44,45,46,47,48,49,50,51,52],inclusive:!0}}};return f})();v.lexer=p;function g(){this.yy={}}return c(g,"Parser"),g.prototype=v,v.Parser=g,new g})();Vt.parser=Vt;var Is=Vt;G.extend(gs);G.extend(Ts);G.extend(Ds);var he={friday:5,saturday:6},tt="",qt="",Xt=void 0,Gt="",pt=[],vt=[],jt=new Map,Ut=[],It=[],kt="",Zt="",Ce=["active","done","crit","milestone","vert"],Qt=[],dt="",xt=!1,Kt=!1,Jt="sunday",$t="saturday",zt=0,$s=c(function(){Ut=[],It=[],kt="",Qt=[],Ct=0,Bt=void 0,Mt=void 0,B=[],tt="",qt="",Zt="",Xt=void 0,Gt="",pt=[],vt=[],xt=!1,Kt=!1,zt=0,jt=new Map,dt="",Be(),Jt="sunday",$t="saturday"},"clear"),Ys=c(function(t){dt=t},"setDiagramId"),Ls=c(function(t){qt=t},"setAxisFormat"),Fs=c(function(){return qt},"getAxisFormat"),As=c(function(t){Xt=t},"setTickInterval"),Os=c(function(){return Xt},"getTickInterval"),Ws=c(function(t){Gt=t},"setTodayMarker"),Ns=c(function(){return Gt},"getTodayMarker"),Ps=c(function(t){tt=t},"setDateFormat"),Rs=c(function(){xt=!0},"enableInclusiveEndDates"),Vs=c(function(){return xt},"endDatesAreInclusive"),zs=c(function(){Kt=!0},"enableTopAxis"),Hs=c(function(){return Kt},"topAxisEnabled"),Bs=c(function(t){Zt=t},"setDisplayMode"),qs=c(function(){return Zt},"getDisplayMode"),Xs=c(function(){return tt},"getDateFormat"),Gs=c(function(t){pt=t.toLowerCase().split(/[\s,]+/)},"setIncludes"),js=c(function(){return pt},"getIncludes"),Us=c(function(t){vt=t.toLowerCase().split(/[\s,]+/)},"setExcludes"),Zs=c(function(){return vt},"getExcludes"),Qs=c(function(){return jt},"getLinks"),Ks=c(function(t){kt=t,Ut.push(t)},"addSection"),Js=c(function(){return Ut},"getSections"),ti=c(function(){let t=me();const s=10;let r=0;for(;!t&&r<s;)t=me(),r++;return It=B,It},"getTasks"),Me=c(function(t,s,r,i){const a=t.format(s.trim()),k=t.format("YYYY-MM-DD");return i.includes(a)||i.includes(k)?!1:r.includes("weekends")&&(t.isoWeekday()===he[$t]||t.isoWeekday()===he[$t]+1)||r.includes(t.format("dddd").toLowerCase())?!0:r.includes(a)||r.includes(k)},"isInvalidDate"),ei=c(function(t){Jt=t},"setWeekday"),si=c(function(){return Jt},"getWeekday"),ii=c(function(t){$t=t},"setWeekend"),Ee=c(function(t,s,r,i){if(!r.length||t.manualEndTime)return;let a;t.startTime instanceof Date?a=G(t.startTime):a=G(t.startTime,s,!0),a=a.add(1,"d");let k;t.endTime instanceof Date?k=G(t.endTime):k=G(t.endTime,s,!0);const[y,x]=ri(a,k,s,r,i);t.endTime=y.toDate(),t.renderEndTime=x},"checkTaskDates"),ri=c(function(t,s,r,i,a){let k=!1,y=null;const x=s.add(1e4,"d");for(;t<=s;){if(k||(y=s.toDate()),k=Me(t,r,i,a),k&&(s=s.add(1,"d"),s>x))throw new Error("Failed to find a valid date that was not excluded by `excludes` after 10,000 iterations.");t=t.add(1,"d")}return[s,y]},"fixTaskDates"),Ht=c(function(t,s,r){if(r=r.trim(),c(x=>{const O=x.trim();return O==="x"||O==="X"},"isTimestampFormat")(s)&&/^\d+$/.test(r))return new Date(Number(r));const k=/^after\s+(?<ids>[\d\w- ]+)/.exec(r);if(k!==null){let x=null;for(const F of k.groups.ids.split(" ")){let w=ct(F);w!==void 0&&(!x||w.endTime>x.endTime)&&(x=w)}if(x)return x.endTime;const O=new Date;return O.setHours(0,0,0,0),O}let y=G(r,s.trim(),!0);if(y.isValid())return y.toDate();{ot.debug("Invalid date:"+r),ot.debug("With date format:"+s.trim());const x=new Date(r);if(x===void 0||isNaN(x.getTime())||x.getFullYear()<-1e4||x.getFullYear()>1e4)throw new Error("Invalid date:"+r);return x}},"getStartDate"),Ie=c(function(t){const s=/^(\d+(?:\.\d+)?)([Mdhmswy]|ms)$/.exec(t.trim());return s!==null?[Number.parseFloat(s[1]),s[2]]:[NaN,"ms"]},"parseDuration"),$e=c(function(t,s,r,i=!1){r=r.trim();const k=/^until\s+(?<ids>[\d\w- ]+)/.exec(r);if(k!==null){let w=null;for(const P of k.groups.ids.split(" ")){let R=ct(P);R!==void 0&&(!w||R.startTime<w.startTime)&&(w=R)}if(w)return w.startTime;const A=new Date;return A.setHours(0,0,0,0),A}let y=G(r,s.trim(),!0);if(y.isValid())return i&&(y=y.add(1,"d")),y.toDate();let x=G(t);const[O,F]=Ie(r);if(!Number.isNaN(O)){const w=x.add(O,F);w.isValid()&&(x=w)}return x.toDate()},"getEndDate"),Ct=0,mt=c(function(t){return t===void 0?(Ct=Ct+1,"task"+Ct):t},"parseId"),ni=c(function(t,s){let r;s.substr(0,1)===":"?r=s.substr(1,s.length):r=s;const i=r.split(","),a={};te(i,a,Ce);for(let y=0;y<i.length;y++)i[y]=i[y].trim();let k="";switch(i.length){case 1:a.id=mt(),a.startTime=t.endTime,k=i[0];break;case 2:a.id=mt(),a.startTime=Ht(void 0,tt,i[0]),k=i[1];break;case 3:a.id=mt(i[0]),a.startTime=Ht(void 0,tt,i[1]),k=i[2];break}return k&&(a.endTime=$e(a.startTime,tt,k,xt),a.manualEndTime=G(k,"YYYY-MM-DD",!0).isValid(),Ee(a,tt,vt,pt)),a},"compileData"),ai=c(function(t,s){let r;s.substr(0,1)===":"?r=s.substr(1,s.length):r=s;const i=r.split(","),a={};te(i,a,Ce);for(let k=0;k<i.length;k++)i[k]=i[k].trim();switch(i.length){case 1:a.id=mt(),a.startTime={type:"prevTaskEnd",id:t},a.endTime={data:i[0]};break;case 2:a.id=mt(),a.startTime={type:"getStartDate",startData:i[0]},a.endTime={data:i[1]};break;case 3:a.id=mt(i[0]),a.startTime={type:"getStartDate",startData:i[1]},a.endTime={data:i[2]};break}return a},"parseData"),Bt,Mt,B=[],Ye={},oi=c(function(t,s){const r={section:kt,type:kt,processed:!1,manualEndTime:!1,renderEndTime:null,raw:{data:s},task:t,classes:[]},i=ai(Mt,s);r.raw.startTime=i.startTime,r.raw.endTime=i.endTime,r.id=i.id,r.prevTaskId=Mt,r.active=i.active,r.done=i.done,r.crit=i.crit,r.milestone=i.milestone,r.vert=i.vert,r.order=zt,zt++;const a=B.push(r);Mt=r.id,Ye[r.id]=a-1},"addTask"),ct=c(function(t){const s=Ye[t];return B[s]},"findTaskById"),ci=c(function(t,s){const r={section:kt,type:kt,description:t,task:t,classes:[]},i=ni(Bt,s);r.startTime=i.startTime,r.endTime=i.endTime,r.id=i.id,r.active=i.active,r.done=i.done,r.crit=i.crit,r.milestone=i.milestone,r.vert=i.vert,Bt=r,It.push(r)},"addTaskOrg"),me=c(function(){const t=c(function(r){const i=B[r];let a="";switch(B[r].raw.startTime.type){case"prevTaskEnd":{const k=ct(i.prevTaskId);i.startTime=k.endTime;break}case"getStartDate":a=Ht(void 0,tt,B[r].raw.startTime.startData),a&&(B[r].startTime=a);break}return B[r].startTime&&(B[r].endTime=$e(B[r].startTime,tt,B[r].raw.endTime.data,xt),B[r].endTime&&(B[r].processed=!0,B[r].manualEndTime=G(B[r].raw.endTime.data,"YYYY-MM-DD",!0).isValid(),Ee(B[r],tt,vt,pt))),B[r].processed},"compileTask");let s=!0;for(const[r,i]of B.entries())t(r),s=s&&i.processed;return s},"compileTasks"),li=c(function(t,s){let r=s;ft().securityLevel!=="loose"&&(r=He.sanitizeUrl(s)),t.split(",").forEach(function(i){ct(i)!==void 0&&(Fe(i,()=>{window.open(r,"_self")}),jt.set(i,r))}),Le(t,"clickable")},"setLink"),Le=c(function(t,s){t.split(",").forEach(function(r){let i=ct(r);i!==void 0&&i.classes.push(s)})},"setClass"),ui=c(function(t,s,r){if(ft().securityLevel!=="loose"||s===void 0)return;let i=[];if(typeof r=="string"){i=r.split(/,(?=(?:(?:[^"]*"){2})*[^"]*$)/);for(let k=0;k<i.length;k++){let y=i[k].trim();y.startsWith('"')&&y.endsWith('"')&&(y=y.substr(1,y.length-2)),i[k]=y}}i.length===0&&i.push(t),ct(t)!==void 0&&Fe(t,()=>{qe.runFunc(s,...i)})},"setClickFun"),Fe=c(function(t,s){Qt.push(function(){const r=dt?`${dt}-${t}`:t,i=document.querySelector(`[id="${r}"]`);i!==null&&i.addEventListener("click",function(){s()})},function(){const r=dt?`${dt}-${t}`:t,i=document.querySelector(`[id="${r}-text"]`);i!==null&&i.addEventListener("click",function(){s()})})},"pushFun"),di=c(function(t,s,r){t.split(",").forEach(function(i){ui(i,s,r)}),Le(t,"clickable")},"setClickEvent"),fi=c(function(t){Qt.forEach(function(s){s(t)})},"bindFunctions"),hi={getConfig:c(()=>ft().gantt,"getConfig"),clear:$s,setDateFormat:Ps,getDateFormat:Xs,enableInclusiveEndDates:Rs,endDatesAreInclusive:Vs,enableTopAxis:zs,topAxisEnabled:Hs,setAxisFormat:Ls,getAxisFormat:Fs,setTickInterval:As,getTickInterval:Os,setTodayMarker:Ws,getTodayMarker:Ns,setAccTitle:Re,getAccTitle:Pe,setDiagramTitle:Ne,getDiagramTitle:We,setDiagramId:Ys,setDisplayMode:Bs,getDisplayMode:qs,setAccDescription:Oe,getAccDescription:Ae,addSection:Ks,getSections:Js,getTasks:ti,addTask:oi,findTaskById:ct,addTaskOrg:ci,setIncludes:Gs,getIncludes:js,setExcludes:Us,getExcludes:Zs,setClickEvent:di,setLink:li,getLinks:Qs,bindFunctions:fi,parseDuration:Ie,isInvalidDate:Me,setWeekday:ei,getWeekday:si,setWeekend:ii};function te(t,s,r){let i=!0;for(;i;)i=!1,r.forEach(function(a){const k="^\\s*"+a+"\\s*$",y=new RegExp(k);t[0].match(y)&&(s[a]=!0,t.shift(1),i=!0)})}c(te,"getTaskTags");G.extend(Es);var mi=c(function(){ot.debug("Something is calling, setConf, remove the call")},"setConf"),ke={monday:os,tuesday:as,wednesday:ns,thursday:rs,friday:is,saturday:ss,sunday:es},ki=c((t,s)=>{let r=[...t].map(()=>-1/0),i=[...t].sort((k,y)=>k.startTime-y.startTime||k.order-y.order),a=0;for(const k of i)for(let y=0;y<r.length;y++)if(k.startTime>=r[y]){r[y]=k.endTime,k.order=y+s,y>a&&(a=y);break}return a},"getMaxIntersections"),st,Pt=1e4,yi=c(function(t,s,r,i){const a=ft().gantt;i.db.setDiagramId(s);const k=ft().securityLevel;let y;k==="sandbox"&&(y=bt("#i"+s));const x=k==="sandbox"?bt(y.nodes()[0].contentDocument.body):bt("body"),O=k==="sandbox"?y.nodes()[0].contentDocument:document,F=O.getElementById(s);st=F.parentElement.offsetWidth,st===void 0&&(st=1200),a.useWidth!==void 0&&(st=a.useWidth);const w=i.db.getTasks();let A=[];for(const u of w)A.push(u.type);A=d(A);const P={};let R=2*a.topPadding;if(i.db.getDisplayMode()==="compact"||a.displayMode==="compact"){const u={};for(const p of w)u[p.section]===void 0?u[p.section]=[p]:u[p.section].push(p);let v=0;for(const p of Object.keys(u)){const g=ki(u[p],v)+1;v+=g,R+=g*(a.barHeight+a.barGap),P[p]=g}}else{R+=w.length*(a.barHeight+a.barGap);for(const u of A)P[u]=w.filter(v=>v.type===u).length}F.setAttribute("viewBox","0 0 "+st+" "+R);const V=x.select(`[id="${s}"]`),M=Ze().domain([Qe(w,function(u){return u.startTime}),Ke(w,function(u){return u.endTime})]).rangeRound([0,st-a.leftPadding-a.rightPadding]);function D(u,v){const p=u.startTime,g=v.startTime;let f=0;return p>g?f=1:p<g&&(f=-1),f}c(D,"taskCompare"),w.sort(D),S(w,st,R),Ve(V,R,st,a.useMaxWidth),V.append("text").text(i.db.getDiagramTitle()).attr("x",st/2).attr("y",a.titleTopMargin).attr("class","titleText");function S(u,v,p){const g=a.barHeight,f=g+a.barGap,o=a.topPadding,l=a.leftPadding,h=Je().domain([0,A.length]).range(["#00B9FA","#F95002"]).interpolate(hs);Y(f,o,l,v,p,u,i.db.getExcludes(),i.db.getIncludes()),q(l,o,v,p),W(u,f,o,l,g,h,v),E(f,o),T(l,o,v,p)}c(S,"makeGantt");function W(u,v,p,g,f,o,l){u.sort((e,_)=>e.vert===_.vert?0:e.vert?1:-1);const m=[...new Set(u.map(e=>e.order))].map(e=>u.find(_=>_.order===e));V.append("g").selectAll("rect").data(m).enter().append("rect").attr("x",0).attr("y",function(e,_){return _=e.order,_*v+p-2}).attr("width",function(){return l-a.rightPadding/2}).attr("height",v).attr("class",function(e){for(const[_,L]of A.entries())if(e.type===L)return"section section"+_%a.numberSectionStyles;return"section section0"}).enter();const b=V.append("g").selectAll("rect").data(u).enter(),n=i.db.getLinks();if(b.append("rect").attr("id",function(e){return s+"-"+e.id}).attr("rx",3).attr("ry",3).attr("x",function(e){return e.milestone?M(e.startTime)+g+.5*(M(e.endTime)-M(e.startTime))-.5*f:M(e.startTime)+g}).attr("y",function(e,_){return _=e.order,e.vert?a.gridLineStartPadding:_*v+p}).attr("width",function(e){return e.milestone?f:e.vert?.08*f:M(e.renderEndTime||e.endTime)-M(e.startTime)}).attr("height",function(e){return e.vert?w.length*(a.barHeight+a.barGap)+a.barHeight*2:f}).attr("transform-origin",function(e,_){return _=e.order,(M(e.startTime)+g+.5*(M(e.endTime)-M(e.startTime))).toString()+"px "+(_*v+p+.5*f).toString()+"px"}).attr("class",function(e){const _="task";let L="";e.classes.length>0&&(L=e.classes.join(" "));let $=0;for(const[X,C]of A.entries())e.type===C&&($=X%a.numberSectionStyles);let I="";return e.active?e.crit?I+=" activeCrit":I=" active":e.done?e.crit?I=" doneCrit":I=" done":e.crit&&(I+=" crit"),I.length===0&&(I=" task"),e.milestone&&(I=" milestone "+I),e.vert&&(I=" vert "+I),I+=$,I+=" "+L,_+I}),b.append("text").attr("id",function(e){return s+"-"+e.id+"-text"}).text(function(e){return e.task}).attr("font-size",a.fontSize).attr("x",function(e){let _=M(e.startTime),L=M(e.renderEndTime||e.endTime);if(e.milestone&&(_+=.5*(M(e.endTime)-M(e.startTime))-.5*f,L=_+f),e.vert)return M(e.startTime)+g;const $=this.getBBox().width;return $>L-_?L+$+1.5*a.leftPadding>l?_+g-5:L+g+5:(L-_)/2+_+g}).attr("y",function(e,_){return e.vert?a.gridLineStartPadding+w.length*(a.barHeight+a.barGap)+60:(_=e.order,_*v+a.barHeight/2+(a.fontSize/2-2)+p)}).attr("text-height",f).attr("class",function(e){const _=M(e.startTime);let L=M(e.endTime);e.milestone&&(L=_+f);const $=this.getBBox().width;let I="";e.classes.length>0&&(I=e.classes.join(" "));let X=0;for(const[Q,rt]of A.entries())e.type===rt&&(X=Q%a.numberSectionStyles);let C="";return e.active&&(e.crit?C="activeCritText"+X:C="activeText"+X),e.done?e.crit?C=C+" doneCritText"+X:C=C+" doneText"+X:e.crit&&(C=C+" critText"+X),e.milestone&&(C+=" milestoneText"),e.vert&&(C+=" vertText"),$>L-_?L+$+1.5*a.leftPadding>l?I+" taskTextOutsideLeft taskTextOutside"+X+" "+C:I+" taskTextOutsideRight taskTextOutside"+X+" "+C+" width-"+$:I+" taskText taskText"+X+" "+C+" width-"+$}),ft().securityLevel==="sandbox"){let e;e=bt("#i"+s);const _=e.nodes()[0].contentDocument;b.filter(function(L){return n.has(L.id)}).each(function(L){var $=_.querySelector("#"+CSS.escape(s+"-"+L.id)),I=_.querySelector("#"+CSS.escape(s+"-"+L.id+"-text"));const X=$.parentNode;var C=_.createElement("a");C.setAttribute("xlink:href",n.get(L.id)),C.setAttribute("target","_top"),X.appendChild(C),C.appendChild($),C.appendChild(I)})}}c(W,"drawRects");function Y(u,v,p,g,f,o,l,h){if(l.length===0&&h.length===0)return;let m,b;for(const{startTime:$,endTime:I}of o)(m===void 0||$<m)&&(m=$),(b===void 0||I>b)&&(b=I);if(!m||!b)return;if(G(b).diff(G(m),"year")>5){ot.warn("The difference between the min and max time is more than 5 years. This will cause performance issues. Skipping drawing exclude days.");return}const n=i.db.getDateFormat(),N=[];let e=null,_=G(m);for(;_.valueOf()<=b;)i.db.isInvalidDate(_,n,l,h)?e?e.end=_:e={start:_,end:_}:e&&(N.push(e),e=null),_=_.add(1,"d");V.append("g").selectAll("rect").data(N).enter().append("rect").attr("id",$=>s+"-exclude-"+$.start.format("YYYY-MM-DD")).attr("x",$=>M($.start.startOf("day"))+p).attr("y",a.gridLineStartPadding).attr("width",$=>M($.end.endOf("day"))-M($.start.startOf("day"))).attr("height",f-v-a.gridLineStartPadding).attr("transform-origin",function($,I){return(M($.start)+p+.5*(M($.end)-M($.start))).toString()+"px "+(I*u+.5*f).toString()+"px"}).attr("class","exclude-range")}c(Y,"drawExcludeDays");function z(u,v,p,g){if(p<=0||u>v)return 1/0;const f=v-u,o=G.duration({[g??"day"]:p}).asMilliseconds();return o<=0?1/0:Math.ceil(f/o)}c(z,"getEstimatedTickCount");function q(u,v,p,g){const f=i.db.getDateFormat(),o=i.db.getAxisFormat();let l;o?l=o:f==="D"?l="%d":l=a.axisFormat??"%Y-%m-%d";let h=ts(M).tickSize(-g+v+a.gridLineStartPadding).tickFormat(se(l));const b=/^([1-9]\d*)(millisecond|second|minute|hour|day|week|month)$/.exec(i.db.getTickInterval()||a.tickInterval);if(b!==null){const n=parseInt(b[1],10);if(isNaN(n)||n<=0)ot.warn(`Invalid tick interval value: "${b[1]}". Skipping custom tick interval.`);else{const N=b[2],e=i.db.getWeekday()||a.weekday,_=M.domain(),L=_[0],$=_[1],I=z(L,$,n,N);if(I>Pt)ot.warn(`The tick interval "${n}${N}" would generate ${I} ticks, which exceeds the maximum allowed (${Pt}). This may indicate an invalid date or time range. Skipping custom tick interval.`);else switch(N){case"millisecond":h.ticks(ce.every(n));break;case"second":h.ticks(oe.every(n));break;case"minute":h.ticks(ae.every(n));break;case"hour":h.ticks(ne.every(n));break;case"day":h.ticks(re.every(n));break;case"week":h.ticks(ke[e].every(n));break;case"month":h.ticks(ie.every(n));break}}}if(V.append("g").attr("class","grid").attr("transform","translate("+u+", "+(g-50)+")").call(h).selectAll("text").style("text-anchor","middle").attr("fill","#000").attr("stroke","none").attr("font-size",10).attr("dy","1em"),i.db.topAxisEnabled()||a.topAxis){let n=cs(M).tickSize(-g+v+a.gridLineStartPadding).tickFormat(se(l));if(b!==null){const N=parseInt(b[1],10);if(isNaN(N)||N<=0)ot.warn(`Invalid tick interval value: "${b[1]}". Skipping custom tick interval.`);else{const e=b[2],_=i.db.getWeekday()||a.weekday,L=M.domain(),$=L[0],I=L[1];if(z($,I,N,e)<=Pt)switch(e){case"millisecond":n.ticks(ce.every(N));break;case"second":n.ticks(oe.every(N));break;case"minute":n.ticks(ae.every(N));break;case"hour":n.ticks(ne.every(N));break;case"day":n.ticks(re.every(N));break;case"week":n.ticks(ke[_].every(N));break;case"month":n.ticks(ie.every(N));break}}}V.append("g").attr("class","grid").attr("transform","translate("+u+", "+v+")").call(n).selectAll("text").style("text-anchor","middle").attr("fill","#000").attr("stroke","none").attr("font-size",10)}}c(q,"makeGrid");function E(u,v){let p=0;const g=Object.keys(P).map(f=>[f,P[f]]);V.append("g").selectAll("text").data(g).enter().append(function(f){const o=f[0].split(ze.lineBreakRegex),l=-(o.length-1)/2,h=O.createElementNS("http://www.w3.org/2000/svg","text");h.setAttribute("dy",l+"em");for(const[m,b]of o.entries()){const n=O.createElementNS("http://www.w3.org/2000/svg","tspan");n.setAttribute("alignment-baseline","central"),n.setAttribute("x","10"),m>0&&n.setAttribute("dy","1em"),n.textContent=b,h.appendChild(n)}return h}).attr("x",10).attr("y",function(f,o){if(o>0)for(let l=0;l<o;l++)return p+=g[o-1][1],f[1]*u/2+p*u+v;else return f[1]*u/2+v}).attr("font-size",a.sectionFontSize).attr("class",function(f){for(const[o,l]of A.entries())if(f[0]===l)return"sectionTitle sectionTitle"+o%a.numberSectionStyles;return"sectionTitle"})}c(E,"vertLabels");function T(u,v,p,g){const f=i.db.getTodayMarker();if(f==="off")return;const o=V.append("g").attr("class","today"),l=new Date,h=o.append("line");h.attr("x1",M(l)+u).attr("x2",M(l)+u).attr("y1",a.titleTopMargin).attr("y2",g-a.titleTopMargin).attr("class","today"),f!==""&&h.attr("style",f.replace(/,/g,";"))}c(T,"drawToday");function d(u){const v={},p=[];for(let g=0,f=u.length;g<f;++g)Object.prototype.hasOwnProperty.call(v,u[g])||(v[u[g]]=!0,p.push(u[g]));return p}c(d,"checkUnique")},"draw"),gi={setConf:mi,draw:yi},pi=c(t=>`
  .mermaid-main-font {
        font-family: ${t.fontFamily};
  }

  .exclude-range {
    fill: ${t.excludeBkgColor};
  }

  .section {
    stroke: none;
    opacity: 0.2;
  }

  .section0 {
    fill: ${t.sectionBkgColor};
  }

  .section2 {
    fill: ${t.sectionBkgColor2};
  }

  .section1,
  .section3 {
    fill: ${t.altSectionBkgColor};
    opacity: 0.2;
  }

  .sectionTitle0 {
    fill: ${t.titleColor};
  }

  .sectionTitle1 {
    fill: ${t.titleColor};
  }

  .sectionTitle2 {
    fill: ${t.titleColor};
  }

  .sectionTitle3 {
    fill: ${t.titleColor};
  }

  .sectionTitle {
    text-anchor: start;
    font-family: ${t.fontFamily};
  }


  /* Grid and axis */

  .grid .tick {
    stroke: ${t.gridColor};
    opacity: 0.8;
    shape-rendering: crispEdges;
  }

  .grid .tick text {
    font-family: ${t.fontFamily};
    fill: ${t.textColor};
  }

  .grid path {
    stroke-width: 0;
  }


  /* Today line */

  .today {
    fill: none;
    stroke: ${t.todayLineColor};
    stroke-width: 2px;
  }


  /* Task styling */

  /* Default task */

  .task {
    stroke-width: 2;
  }

  .taskText {
    text-anchor: middle;
    font-family: ${t.fontFamily};
  }

  .taskTextOutsideRight {
    fill: ${t.taskTextDarkColor};
    text-anchor: start;
    font-family: ${t.fontFamily};
  }

  .taskTextOutsideLeft {
    fill: ${t.taskTextDarkColor};
    text-anchor: end;
  }


  /* Special case clickable */

  .task.clickable {
    cursor: pointer;
  }

  .taskText.clickable {
    cursor: pointer;
    fill: ${t.taskTextClickableColor} !important;
    font-weight: bold;
  }

  .taskTextOutsideLeft.clickable {
    cursor: pointer;
    fill: ${t.taskTextClickableColor} !important;
    font-weight: bold;
  }

  .taskTextOutsideRight.clickable {
    cursor: pointer;
    fill: ${t.taskTextClickableColor} !important;
    font-weight: bold;
  }


  /* Specific task settings for the sections*/

  .taskText0,
  .taskText1,
  .taskText2,
  .taskText3 {
    fill: ${t.taskTextColor};
  }

  .task0,
  .task1,
  .task2,
  .task3 {
    fill: ${t.taskBkgColor};
    stroke: ${t.taskBorderColor};
  }

  .taskTextOutside0,
  .taskTextOutside2
  {
    fill: ${t.taskTextOutsideColor};
  }

  .taskTextOutside1,
  .taskTextOutside3 {
    fill: ${t.taskTextOutsideColor};
  }


  /* Active task */

  .active0,
  .active1,
  .active2,
  .active3 {
    fill: ${t.activeTaskBkgColor};
    stroke: ${t.activeTaskBorderColor};
  }

  .activeText0,
  .activeText1,
  .activeText2,
  .activeText3 {
    fill: ${t.taskTextDarkColor} !important;
  }


  /* Completed task */

  .done0,
  .done1,
  .done2,
  .done3 {
    stroke: ${t.doneTaskBorderColor};
    fill: ${t.doneTaskBkgColor};
    stroke-width: 2;
  }

  .doneText0,
  .doneText1,
  .doneText2,
  .doneText3 {
    fill: ${t.taskTextDarkColor} !important;
  }

  /* Done task text displayed outside the bar sits against the diagram background,
     not against the done-task bar, so it must use the outside/contrast color. */
  .doneText0.taskTextOutsideLeft,
  .doneText0.taskTextOutsideRight,
  .doneText1.taskTextOutsideLeft,
  .doneText1.taskTextOutsideRight,
  .doneText2.taskTextOutsideLeft,
  .doneText2.taskTextOutsideRight,
  .doneText3.taskTextOutsideLeft,
  .doneText3.taskTextOutsideRight {
    fill: ${t.taskTextOutsideColor} !important;
  }


  /* Tasks on the critical line */

  .crit0,
  .crit1,
  .crit2,
  .crit3 {
    stroke: ${t.critBorderColor};
    fill: ${t.critBkgColor};
    stroke-width: 2;
  }

  .activeCrit0,
  .activeCrit1,
  .activeCrit2,
  .activeCrit3 {
    stroke: ${t.critBorderColor};
    fill: ${t.activeTaskBkgColor};
    stroke-width: 2;
  }

  .doneCrit0,
  .doneCrit1,
  .doneCrit2,
  .doneCrit3 {
    stroke: ${t.critBorderColor};
    fill: ${t.doneTaskBkgColor};
    stroke-width: 2;
    cursor: pointer;
    shape-rendering: crispEdges;
  }

  .milestone {
    transform: rotate(45deg) scale(0.8,0.8);
  }

  .milestoneText {
    font-style: italic;
  }
  .doneCritText0,
  .doneCritText1,
  .doneCritText2,
  .doneCritText3 {
    fill: ${t.taskTextDarkColor} !important;
  }

  /* Done-crit task text outside the bar — same reasoning as doneText above. */
  .doneCritText0.taskTextOutsideLeft,
  .doneCritText0.taskTextOutsideRight,
  .doneCritText1.taskTextOutsideLeft,
  .doneCritText1.taskTextOutsideRight,
  .doneCritText2.taskTextOutsideLeft,
  .doneCritText2.taskTextOutsideRight,
  .doneCritText3.taskTextOutsideLeft,
  .doneCritText3.taskTextOutsideRight {
    fill: ${t.taskTextOutsideColor} !important;
  }

  .vert {
    stroke: ${t.vertLineColor};
  }

  .vertText {
    font-size: 15px;
    text-anchor: middle;
    fill: ${t.vertLineColor} !important;
  }

  .activeCritText0,
  .activeCritText1,
  .activeCritText2,
  .activeCritText3 {
    fill: ${t.taskTextDarkColor} !important;
  }

  .titleText {
    text-anchor: middle;
    font-size: 18px;
    fill: ${t.titleColor||t.textColor};
    font-family: ${t.fontFamily};
  }
`,"getStyles"),vi=pi,Di={parser:Is,db:hi,renderer:gi,styles:vi};export{Di as diagram};
//# sourceMappingURL=ganttDiagram-6RSMTGT7-XeI4yLY0.js.map
