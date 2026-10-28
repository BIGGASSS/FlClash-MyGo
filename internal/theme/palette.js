var FlClashPalette=(()=>{var Dt=Object.defineProperty;var $t=Object.getOwnPropertyDescriptor;var Kt=Object.getOwnPropertyNames;var Jt=Object.prototype.hasOwnProperty;var Xt=(t,e)=>{for(var r in e)Dt(t,r,{get:e[r],enumerable:!0})},Zt=(t,e,r,n)=>{if(e&&typeof e=="object"||typeof e=="function")for(let a of Kt(e))!Jt.call(t,a)&&a!==r&&Dt(t,a,{get:()=>e[a],enumerable:!(n=$t(e,a))||n.enumerable});return t};var Qt=t=>Zt(Dt({},"__esModule",{value:!0}),t);var he={};Xt(he,{palette:()=>ce});/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */function v(t){return t<0?-1:t===0?0:1}function Z(t,e,r){return(1-r)*t+r*e}function Lt(t,e,r){return r<t?t:r>e?e:r}function et(t,e,r){return r<t?t:r>e?e:r}function rt(t){return t=t%360,t<0&&(t=t+360),t}function L(t){return t=t%360,t<0&&(t=t+360),t}function Tt(t,e){return 180-Math.abs(Math.abs(t-e)-180)}function ot(t,e){let r=t[0]*e[0][0]+t[1]*e[0][1]+t[2]*e[0][2],n=t[0]*e[1][0]+t[1]*e[1][1]+t[2]*e[1][2],a=t[0]*e[2][0]+t[1]*e[2][1]+t[2]*e[2][2];return[r,n,a]}/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var Vt=[[.41233895,.35762064,.18051042],[.2126,.7152,.0722],[.01932141,.11916382,.95034478]],te=[[3.2413774792388685,-1.5376652402851851,-.49885366846268053],[-.9691452513005321,1.8758853451067872,.04156585616912061],[.05562093689691305,-.20395524564742123,1.0571799111220335]],zt=[95.047,100,108.883];function gt(t,e,r){return(255<<24|(t&255)<<16|(e&255)<<8|r&255)>>>0}function wt(t){let e=Q(t[0]),r=Q(t[1]),n=Q(t[2]);return gt(e,r,n)}function pt(t){return t>>16&255}function dt(t){return t>>8&255}function yt(t){return t&255}function Nt(t,e,r){let n=te,a=n[0][0]*t+n[0][1]*e+n[0][2]*r,s=n[1][0]*t+n[1][1]*e+n[1][2]*r,i=n[2][0]*t+n[2][1]*e+n[2][2]*r,c=Q(a),h=Q(s),m=Q(i);return gt(c,h,m)}function ee(t){let e=Y(pt(t)),r=Y(dt(t)),n=Y(yt(t));return ot([e,r,n],Vt)}function St(t){let e=Y(pt(t)),r=Y(dt(t)),n=Y(yt(t)),a=Vt,s=a[0][0]*e+a[0][1]*r+a[0][2]*n,i=a[1][0]*e+a[1][1]*r+a[1][2]*n,c=a[2][0]*e+a[2][1]*r+a[2][2]*n,h=zt,m=s/h[0],u=i/h[1],y=c/h[2],p=at(m),f=at(u),g=at(y),l=116*f-16,x=500*(p-f),k=200*(f-g);return[l,x,k]}function Ut(t){let e=G(t),r=Q(e);return gt(r,r,r)}function Ct(t){let e=ee(t)[1];return 116*at(e/100)-16}function G(t){return 100*re((t+16)/116)}function st(t){return at(t/100)*116-16}function Y(t){let e=t/255;return e<=.040449936?e/12.92*100:Math.pow((e+.055)/1.055,2.4)*100}function Q(t){let e=t/100,r=0;return e<=.0031308?r=e*12.92:r=1.055*Math.pow(e,1/2.4)-.055,Lt(0,255,Math.round(r*255))}function Gt(){return zt}function at(t){let e=.008856451679035631,r=24389/27;return t>e?Math.pow(t,1/3):(r*t+16)/116}function re(t){let e=.008856451679035631,r=24389/27,n=t*t*t;return n>e?n:(116*t-16)/r}/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var N=class t{static make(e=Gt(),r=200/Math.PI*G(50)/100,n=50,a=2,s=!1){let i=e,c=i[0]*.401288+i[1]*.650173+i[2]*-.051461,h=i[0]*-.250268+i[1]*1.204414+i[2]*.045854,m=i[0]*-.002079+i[1]*.048952+i[2]*.953127,u=.8+a/10,y=u>=.9?Z(.59,.69,(u-.9)*10):Z(.525,.59,(u-.8)*10),p=s?1:u*(1-1/3.6*Math.exp((-r-42)/92));p=p>1?1:p<0?0:p;let f=u,g=[p*(100/c)+1-p,p*(100/h)+1-p,p*(100/m)+1-p],l=1/(5*r+1),x=l*l*l*l,k=1-x,M=x*r+.1*k*k*Math.cbrt(5*r),b=G(n)/e[1],R=1.48+Math.sqrt(b),A=.725/Math.pow(b,.2),H=A,D=[Math.pow(M*g[0]*c/100,.42),Math.pow(M*g[1]*h/100,.42),Math.pow(M*g[2]*m/100,.42)],S=[400*D[0]/(D[0]+27.13),400*D[1]/(D[1]+27.13),400*D[2]/(D[2]+27.13)],O=(2*S[0]+S[1]+.05*S[2])*A;return new t(b,O,A,H,y,f,g,M,Math.pow(M,.25),R)}constructor(e,r,n,a,s,i,c,h,m,u){this.n=e,this.aw=r,this.nbb=n,this.ncb=a,this.c=s,this.nc=i,this.rgbD=c,this.fl=h,this.fLRoot=m,this.z=u}};N.DEFAULT=N.make();/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var W=class t{constructor(e,r,n,a,s,i,c,h,m){this.hue=e,this.chroma=r,this.j=n,this.q=a,this.m=s,this.s=i,this.jstar=c,this.astar=h,this.bstar=m}distance(e){let r=this.jstar-e.jstar,n=this.astar-e.astar,a=this.bstar-e.bstar,s=Math.sqrt(r*r+n*n+a*a);return 1.41*Math.pow(s,.63)}static fromInt(e){return t.fromIntInViewingConditions(e,N.DEFAULT)}static fromIntInViewingConditions(e,r){let n=(e&16711680)>>16,a=(e&65280)>>8,s=e&255,i=Y(n),c=Y(a),h=Y(s),m=.41233895*i+.35762064*c+.18051042*h,u=.2126*i+.7152*c+.0722*h,y=.01932141*i+.11916382*c+.95034478*h,p=.401288*m+.650173*u-.051461*y,f=-.250268*m+1.204414*u+.045854*y,g=-.002079*m+.048952*u+.953127*y,l=r.rgbD[0]*p,x=r.rgbD[1]*f,k=r.rgbD[2]*g,M=Math.pow(r.fl*Math.abs(l)/100,.42),b=Math.pow(r.fl*Math.abs(x)/100,.42),R=Math.pow(r.fl*Math.abs(k)/100,.42),A=v(l)*400*M/(M+27.13),H=v(x)*400*b/(b+27.13),D=v(k)*400*R/(R+27.13),S=(11*A+-12*H+D)/11,O=(A+H-2*D)/9,w=(20*A+20*H+21*D)/20,$=(40*A+20*H+D)/20,_=Math.atan2(O,S)*180/Math.PI,U=_<0?_+360:_>=360?_-360:_,lt=U*Math.PI/180,ut=$*r.nbb,X=100*Math.pow(ut/r.aw,r.c*r.z),mt=4/r.c*Math.sqrt(X/100)*(r.aw+4)*r.fLRoot,At=U<20.14?U+360:U,kt=.25*(Math.cos(At*Math.PI/180+2)+3.8),Mt=5e4/13*kt*r.nc*r.ncb*Math.sqrt(S*S+O*O)/(w+.305),ft=Math.pow(Mt,.9)*Math.pow(1.64-Math.pow(.29,r.n),.73),Ot=ft*Math.sqrt(X/100),Et=Ot*r.fLRoot,jt=50*Math.sqrt(ft*r.c/(r.aw+4)),_t=(1+100*.007)*X/(1+.007*X),vt=1/.0228*Math.log(1+.0228*Et),Yt=vt*Math.cos(lt),Wt=vt*Math.sin(lt);return new t(U,Ot,X,mt,Et,jt,_t,Yt,Wt)}static fromJch(e,r,n){return t.fromJchInViewingConditions(e,r,n,N.DEFAULT)}static fromJchInViewingConditions(e,r,n,a){let s=4/a.c*Math.sqrt(e/100)*(a.aw+4)*a.fLRoot,i=r*a.fLRoot,c=r/Math.sqrt(e/100),h=50*Math.sqrt(c*a.c/(a.aw+4)),m=n*Math.PI/180,u=(1+100*.007)*e/(1+.007*e),y=1/.0228*Math.log(1+.0228*i),p=y*Math.cos(m),f=y*Math.sin(m);return new t(n,r,e,s,i,h,u,p,f)}static fromUcs(e,r,n){return t.fromUcsInViewingConditions(e,r,n,N.DEFAULT)}static fromUcsInViewingConditions(e,r,n,a){let s=r,i=n,c=Math.sqrt(s*s+i*i),m=(Math.exp(c*.0228)-1)/.0228/a.fLRoot,u=Math.atan2(i,s)*(180/Math.PI);u<0&&(u+=360);let y=e/(1-(e-100)*.007);return t.fromJchInViewingConditions(y,m,u,a)}toInt(){return this.viewed(N.DEFAULT)}viewed(e){let r=this.chroma===0||this.j===0?0:this.chroma/Math.sqrt(this.j/100),n=Math.pow(r/Math.pow(1.64-Math.pow(.29,e.n),.73),1/.9),a=this.hue*Math.PI/180,s=.25*(Math.cos(a+2)+3.8),i=e.aw*Math.pow(this.j/100,1/e.c/e.z),c=s*(5e4/13)*e.nc*e.ncb,h=i/e.nbb,m=Math.sin(a),u=Math.cos(a),y=23*(h+.305)*n/(23*c+11*n*u+108*n*m),p=y*u,f=y*m,g=(460*h+451*p+288*f)/1403,l=(460*h-891*p-261*f)/1403,x=(460*h-220*p-6300*f)/1403,k=Math.max(0,27.13*Math.abs(g)/(400-Math.abs(g))),M=v(g)*(100/e.fl)*Math.pow(k,1/.42),b=Math.max(0,27.13*Math.abs(l)/(400-Math.abs(l))),R=v(l)*(100/e.fl)*Math.pow(b,1/.42),A=Math.max(0,27.13*Math.abs(x)/(400-Math.abs(x))),H=v(x)*(100/e.fl)*Math.pow(A,1/.42),D=M/e.rgbD[0],S=R/e.rgbD[1],O=H/e.rgbD[2],w=1.86206786*D-1.01125463*S+.14918677*O,$=.38752654*D+.62144744*S-.00897398*O,J=-.0158415*D-.03412294*S+1.04996444*O;return Nt(w,$,J)}static fromXyzInViewingConditions(e,r,n,a){let s=.401288*e+.650173*r-.051461*n,i=-.250268*e+1.204414*r+.045854*n,c=-.002079*e+.048952*r+.953127*n,h=a.rgbD[0]*s,m=a.rgbD[1]*i,u=a.rgbD[2]*c,y=Math.pow(a.fl*Math.abs(h)/100,.42),p=Math.pow(a.fl*Math.abs(m)/100,.42),f=Math.pow(a.fl*Math.abs(u)/100,.42),g=v(h)*400*y/(y+27.13),l=v(m)*400*p/(p+27.13),x=v(u)*400*f/(f+27.13),k=(11*g+-12*l+x)/11,M=(g+l-2*x)/9,b=(20*g+20*l+21*x)/20,R=(40*g+20*l+x)/20,H=Math.atan2(M,k)*180/Math.PI,D=H<0?H+360:H>=360?H-360:H,S=D*Math.PI/180,O=R*a.nbb,w=100*Math.pow(O/a.aw,a.c*a.z),$=4/a.c*Math.sqrt(w/100)*(a.aw+4)*a.fLRoot,J=D<20.14?D+360:D,_=1/4*(Math.cos(J*Math.PI/180+2)+3.8),lt=5e4/13*_*a.nc*a.ncb*Math.sqrt(k*k+M*M)/(b+.305),ut=Math.pow(lt,.9)*Math.pow(1.64-Math.pow(.29,a.n),.73),X=ut*Math.sqrt(w/100),mt=X*a.fLRoot,At=50*Math.sqrt(ut*a.c/(a.aw+4)),kt=(1+100*.007)*w/(1+.007*w),It=Math.log(1+.0228*mt)/.0228,Mt=It*Math.cos(S),ft=It*Math.sin(S);return new t(D,X,w,$,mt,At,kt,Mt,ft)}xyzInViewingConditions(e){let r=this.chroma===0||this.j===0?0:this.chroma/Math.sqrt(this.j/100),n=Math.pow(r/Math.pow(1.64-Math.pow(.29,e.n),.73),1/.9),a=this.hue*Math.PI/180,s=.25*(Math.cos(a+2)+3.8),i=e.aw*Math.pow(this.j/100,1/e.c/e.z),c=s*(5e4/13)*e.nc*e.ncb,h=i/e.nbb,m=Math.sin(a),u=Math.cos(a),y=23*(h+.305)*n/(23*c+11*n*u+108*n*m),p=y*u,f=y*m,g=(460*h+451*p+288*f)/1403,l=(460*h-891*p-261*f)/1403,x=(460*h-220*p-6300*f)/1403,k=Math.max(0,27.13*Math.abs(g)/(400-Math.abs(g))),M=v(g)*(100/e.fl)*Math.pow(k,1/.42),b=Math.max(0,27.13*Math.abs(l)/(400-Math.abs(l))),R=v(l)*(100/e.fl)*Math.pow(b,1/.42),A=Math.max(0,27.13*Math.abs(x)/(400-Math.abs(x))),H=v(x)*(100/e.fl)*Math.pow(A,1/.42),D=M/e.rgbD[0],S=R/e.rgbD[1],O=H/e.rgbD[2],w=1.86206786*D-1.01125463*S+.14918677*O,$=.38752654*D+.62144744*S-.00897398*O,J=-.0158415*D-.03412294*S+1.04996444*O;return[w,$,J]}};/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var j=class t{static sanitizeRadians(e){return(e+Math.PI*8)%(Math.PI*2)}static trueDelinearized(e){let r=e/100,n=0;return r<=.0031308?n=r*12.92:n=1.055*Math.pow(r,1/2.4)-.055,n*255}static chromaticAdaptation(e){let r=Math.pow(Math.abs(e),.42);return v(e)*400*r/(r+27.13)}static hueOf(e){let r=ot(e,t.SCALED_DISCOUNT_FROM_LINRGB),n=t.chromaticAdaptation(r[0]),a=t.chromaticAdaptation(r[1]),s=t.chromaticAdaptation(r[2]),i=(11*n+-12*a+s)/11,c=(n+a-2*s)/9;return Math.atan2(c,i)}static areInCyclicOrder(e,r,n){let a=t.sanitizeRadians(r-e),s=t.sanitizeRadians(n-e);return a<s}static intercept(e,r,n){return(r-e)/(n-e)}static lerpPoint(e,r,n){return[e[0]+(n[0]-e[0])*r,e[1]+(n[1]-e[1])*r,e[2]+(n[2]-e[2])*r]}static setCoordinate(e,r,n,a){let s=t.intercept(e[a],r,n[a]);return t.lerpPoint(e,s,n)}static isBounded(e){return 0<=e&&e<=100}static nthVertex(e,r){let n=t.Y_FROM_LINRGB[0],a=t.Y_FROM_LINRGB[1],s=t.Y_FROM_LINRGB[2],i=r%4<=1?0:100,c=r%2===0?0:100;if(r<4){let h=i,m=c,u=(e-h*a-m*s)/n;return t.isBounded(u)?[u,h,m]:[-1,-1,-1]}else if(r<8){let h=i,m=c,u=(e-m*n-h*s)/a;return t.isBounded(u)?[m,u,h]:[-1,-1,-1]}else{let h=i,m=c,u=(e-h*n-m*a)/s;return t.isBounded(u)?[h,m,u]:[-1,-1,-1]}}static bisectToSegment(e,r){let n=[-1,-1,-1],a=n,s=0,i=0,c=!1,h=!0;for(let m=0;m<12;m++){let u=t.nthVertex(e,m);if(u[0]<0)continue;let y=t.hueOf(u);if(!c){n=u,a=u,s=y,i=y,c=!0;continue}(h||t.areInCyclicOrder(s,y,i))&&(h=!1,t.areInCyclicOrder(s,r,y)?(a=u,i=y):(n=u,s=y))}return[n,a]}static midpoint(e,r){return[(e[0]+r[0])/2,(e[1]+r[1])/2,(e[2]+r[2])/2]}static criticalPlaneBelow(e){return Math.floor(e-.5)}static criticalPlaneAbove(e){return Math.ceil(e-.5)}static bisectToLimit(e,r){let n=t.bisectToSegment(e,r),a=n[0],s=t.hueOf(a),i=n[1];for(let c=0;c<3;c++)if(a[c]!==i[c]){let h=-1,m=255;a[c]<i[c]?(h=t.criticalPlaneBelow(t.trueDelinearized(a[c])),m=t.criticalPlaneAbove(t.trueDelinearized(i[c]))):(h=t.criticalPlaneAbove(t.trueDelinearized(a[c])),m=t.criticalPlaneBelow(t.trueDelinearized(i[c])));for(let u=0;u<8&&!(Math.abs(m-h)<=1);u++){let y=Math.floor((h+m)/2),p=t.CRITICAL_PLANES[y],f=t.setCoordinate(a,p,i,c),g=t.hueOf(f);t.areInCyclicOrder(s,r,g)?(i=f,m=y):(a=f,s=g,h=y)}}return t.midpoint(a,i)}static inverseChromaticAdaptation(e){let r=Math.abs(e),n=Math.max(0,27.13*r/(400-r));return v(e)*Math.pow(n,1/.42)}static findResultByJ(e,r,n){let a=Math.sqrt(n)*11,s=N.DEFAULT,i=1/Math.pow(1.64-Math.pow(.29,s.n),.73),h=.25*(Math.cos(e+2)+3.8)*(5e4/13)*s.nc*s.ncb,m=Math.sin(e),u=Math.cos(e);for(let y=0;y<5;y++){let p=a/100,f=r===0||a===0?0:r/Math.sqrt(p),g=Math.pow(f*i,1/.9),x=s.aw*Math.pow(p,1/s.c/s.z)/s.nbb,k=23*(x+.305)*g/(23*h+11*g*u+108*g*m),M=k*u,b=k*m,R=(460*x+451*M+288*b)/1403,A=(460*x-891*M-261*b)/1403,H=(460*x-220*M-6300*b)/1403,D=t.inverseChromaticAdaptation(R),S=t.inverseChromaticAdaptation(A),O=t.inverseChromaticAdaptation(H),w=ot([D,S,O],t.LINRGB_FROM_SCALED_DISCOUNT);if(w[0]<0||w[1]<0||w[2]<0)return 0;let $=t.Y_FROM_LINRGB[0],J=t.Y_FROM_LINRGB[1],_=t.Y_FROM_LINRGB[2],U=$*w[0]+J*w[1]+_*w[2];if(U<=0)return 0;if(y===4||Math.abs(U-n)<.002)return w[0]>100.01||w[1]>100.01||w[2]>100.01?0:wt(w);a=a-(U-n)*a/(2*U)}return 0}static solveToInt(e,r,n){if(r<1e-4||n<1e-4||n>99.9999)return Ut(n);e=L(e);let a=e/180*Math.PI,s=G(n),i=t.findResultByJ(a,r,s);if(i!==0)return i;let c=t.bisectToLimit(s,a);return wt(c)}static solveToCam(e,r,n){return W.fromInt(t.solveToInt(e,r,n))}};j.SCALED_DISCOUNT_FROM_LINRGB=[[.001200833568784504,.002389694492170889,.0002795742885861124],[.0005891086651375999,.0029785502573438758,.0003270666104008398],[.00010146692491640572,.0005364214359186694,.0032979401770712076]];j.LINRGB_FROM_SCALED_DISCOUNT=[[1373.2198709594231,-1100.4251190754821,-7.278681089101213],[-271.815969077903,559.6580465940733,-32.46047482791194],[1.9622899599665666,-57.173814538844006,308.7233197812385]];j.Y_FROM_LINRGB=[.2126,.7152,.0722];j.CRITICAL_PLANES=[.015176349177441876,.045529047532325624,.07588174588720938,.10623444424209313,.13658714259697685,.16693984095186062,.19729253930674434,.2276452376616281,.2579979360165119,.28835063437139563,.3188300904430532,.350925934958123,.3848314933096426,.42057480301049466,.458183274052838,.4976837250274023,.5391024159806381,.5824650784040898,.6277969426914107,.6751227633498623,.7244668422128921,.775853049866786,.829304845476233,.8848452951698498,.942497089126609,1.0022825574869039,1.0642236851973577,1.1283421258858297,1.1946592148522128,1.2631959812511864,1.3339731595349034,1.407011200216447,1.4823302800086415,1.5599503113873272,1.6398909516233677,1.7221716113234105,1.8068114625156377,1.8938294463134073,1.9832442801866852,2.075074464868551,2.1693382909216234,2.2660538449872063,2.36523901573795,2.4669114995532007,2.5710888059345764,2.6777882626779785,2.7870270208169257,2.898822059350997,3.0131901897720907,3.1301480604002863,3.2497121605402226,3.3718988244681087,3.4967242352587946,3.624204428461639,3.754355295633311,3.887192587735158,4.022731918402185,4.160988767090289,4.301978482107941,4.445716283538092,4.592217266055746,4.741496401646282,4.893568542229298,5.048448422192488,5.20615066083972,5.3666897647573375,5.5300801301023865,5.696336044816294,5.865471690767354,6.037501145825082,6.212438385869475,6.390297286737924,6.571091626112461,6.7548350853498045,6.941541251256611,7.131223617812143,7.323895587840543,7.5195704746346665,7.7182615035334345,7.919981813454504,8.124744458384042,8.332562408825165,8.543448553206703,8.757415699253682,8.974476575321063,9.194643831691977,9.417930041841839,9.644347703669503,9.873909240696694,10.106627003236781,10.342513269534024,10.58158024687427,10.8238400726681,11.069304815507364,11.317986476196008,11.569896988756009,11.825048221409341,12.083451977536606,12.345119996613247,12.610063955123938,12.878295467455942,13.149826086772048,13.42466730586372,13.702830557985108,13.984327217668513,14.269168601521828,14.55736596900856,14.848930523210871,15.143873411576273,15.44220572664832,15.743938506781891,16.04908273684337,16.35764934889634,16.66964922287304,16.985093187232053,17.30399201960269,17.62635644741625,17.95219714852476,18.281524751807332,18.614349837764564,18.95068293910138,19.290534541298456,19.633915083172692,19.98083495742689,20.331304511189067,20.685334046541502,21.042933821039977,21.404114048223256,21.76888489811322,22.137256497705877,22.50923893145328,22.884842241736916,23.264076429332462,23.6469514538663,24.033477234264016,24.42366364919083,24.817520537484558,25.21505769858089,25.61628489293138,26.021211842414342,26.429848230738664,26.842203703840827,27.258287870275353,27.678110301598522,28.10168053274597,28.529008062403893,28.96010235337422,29.39497283293396,29.83362889318845,30.276079891419332,30.722335150426627,31.172403958865512,31.62629557157785,32.08401920991837,32.54558406207592,33.010999283389665,33.4802739966603,33.953417292456834,34.430438229418264,34.911345834551085,35.39614910352207,35.88485700094671,36.37747846067349,36.87402238606382,37.37449765026789,37.87891309649659,38.38727753828926,38.89959975977785,39.41588851594697,39.93615253289054,40.460400508064545,40.98864111053629,41.520882981230194,42.05713473317016,42.597404951718396,43.141702194811224,43.6900349931913,44.24241185063697,44.798841244188324,45.35933162437017,45.92389141541209,46.49252901546552,47.065252796817916,47.64207110610409,48.22299226451468,48.808024568002054,49.3971762874833,49.9904556690408,50.587870934119984,51.189430279724725,51.79514187861014,52.40501387947288,53.0190544071392,53.637271562750364,54.259673423945976,54.88626804504493,55.517063457223934,56.15206766869424,56.79128866487574,57.43473440856916,58.08241284012621,58.734331877617365,59.39049941699807,60.05092333227251,60.715611475655585,61.38457167773311,62.057811747619894,62.7353394731159,63.417162620860914,64.10328893648692,64.79372614476921,65.48848194977529,66.18756403501224,66.89098006357258,67.59873767827808,68.31084450182222,69.02730813691093,69.74813616640164,70.47333615344107,71.20291564160104,71.93688215501312,72.67524319850172,73.41800625771542,74.16517879925733,74.9167682708136,75.67278210128072,76.43322770089146,77.1981124613393,77.96744375590167,78.74122893956174,79.51947534912904,80.30219030335869,81.08938110306934,81.88105503125999,82.67721935322541,83.4778813166706,84.28304815182372,85.09272707154808,85.90692527145302,86.72564993000343,87.54890820862819,88.3767072518277,89.2090541872801,90.04595612594655,90.88742016217518,91.73345337380438,92.58406282226491,93.43925555268066,94.29903859396902,95.16341895893969,96.03240364439274,96.9059996312159,97.78421388448044,98.6670533535366,99.55452497210776];/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var I=class t{static from(e,r,n){return new t(j.solveToInt(e,r,n))}static fromInt(e){return new t(e)}toInt(){return this.argb}get hue(){return this.internalHue}set hue(e){this.setInternalState(j.solveToInt(e,this.internalChroma,this.internalTone))}get chroma(){return this.internalChroma}set chroma(e){this.setInternalState(j.solveToInt(this.internalHue,e,this.internalTone))}get tone(){return this.internalTone}set tone(e){this.setInternalState(j.solveToInt(this.internalHue,this.internalChroma,e))}constructor(e){this.argb=e;let r=W.fromInt(e);this.internalHue=r.hue,this.internalChroma=r.chroma,this.internalTone=Ct(e),this.argb=e}setInternalState(e){let r=W.fromInt(e);this.internalHue=r.hue,this.internalChroma=r.chroma,this.internalTone=Ct(e),this.argb=e}inViewingConditions(e){let n=W.fromInt(this.toInt()).xyzInViewingConditions(e),a=W.fromXyzInViewingConditions(n[0],n[1],n[2],N.make());return t.from(a.hue,a.chroma,st(n[1]))}};/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var z=class t{static ratioOfTones(e,r){return e=et(0,100,e),r=et(0,100,r),t.ratioOfYs(G(e),G(r))}static ratioOfYs(e,r){let n=e>r?e:r,a=n===r?e:r;return(n+5)/(a+5)}static lighter(e,r){if(e<0||e>100)return-1;let n=G(e),a=r*(n+5)-5,s=t.ratioOfYs(a,n),i=Math.abs(s-r);if(s<r&&i>.04)return-1;let c=st(a)+.4;return c<0||c>100?-1:c}static darker(e,r){if(e<0||e>100)return-1;let n=G(e),a=(n+5)/r-5,s=t.ratioOfYs(n,a),i=Math.abs(s-r);if(s<r&&i>.04)return-1;let c=st(a)-.4;return c<0||c>100?-1:c}static lighterUnsafe(e,r){let n=t.lighter(e,r);return n<0?100:n}static darkerUnsafe(e,r){let n=t.darker(e,r);return n<0?0:n}};/**
 * @license
 * Copyright 2023 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var tt=class t{static isDisliked(e){let r=Math.round(e.hue)>=90&&Math.round(e.hue)<=111,n=Math.round(e.chroma)>16,a=Math.round(e.tone)<65;return r&&n&&a}static fixIfDisliked(e){return t.isDisliked(e)?I.from(e.hue,e.chroma,70):e}};/**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var d=class t{static fromPalette(e){return new t(e.name??"",e.palette,e.tone,e.isBackground??!1,e.background,e.secondBackground,e.contrastCurve,e.toneDeltaPair)}constructor(e,r,n,a,s,i,c,h){if(this.name=e,this.palette=r,this.tone=n,this.isBackground=a,this.background=s,this.secondBackground=i,this.contrastCurve=c,this.toneDeltaPair=h,this.hctCache=new Map,!s&&i)throw new Error(`Color ${e} has secondBackgrounddefined, but background is not defined.`);if(!s&&c)throw new Error(`Color ${e} has contrastCurvedefined, but background is not defined.`);if(s&&!c)throw new Error(`Color ${e} has backgrounddefined, but contrastCurve is not defined.`)}getArgb(e){return this.getHct(e).toInt()}getHct(e){let r=this.hctCache.get(e);if(r!=null)return r;let n=this.getTone(e),a=this.palette(e).getHct(n);return this.hctCache.size>4&&this.hctCache.clear(),this.hctCache.set(e,a),a}getTone(e){let r=e.contrastLevel<0;if(this.toneDeltaPair){let n=this.toneDeltaPair(e),a=n.roleA,s=n.roleB,i=n.delta,c=n.polarity,h=n.stayTogether,u=this.background(e).getTone(e),y=c==="nearer"||c==="lighter"&&!e.isDark||c==="darker"&&e.isDark,p=y?a:s,f=y?s:a,g=this.name===p.name,l=e.isDark?1:-1,x=p.contrastCurve.get(e.contrastLevel),k=f.contrastCurve.get(e.contrastLevel),M=p.tone(e),b=z.ratioOfTones(u,M)>=x?M:t.foregroundTone(u,x),R=f.tone(e),A=z.ratioOfTones(u,R)>=k?R:t.foregroundTone(u,k);return r&&(b=t.foregroundTone(u,x),A=t.foregroundTone(u,k)),(A-b)*l>=i||(A=et(0,100,b+i*l),(A-b)*l>=i||(b=et(0,100,A-i*l))),50<=b&&b<60?l>0?(b=60,A=Math.max(A,b+i*l)):(b=49,A=Math.min(A,b+i*l)):50<=A&&A<60&&(h?l>0?(b=60,A=Math.max(A,b+i*l)):(b=49,A=Math.min(A,b+i*l)):l>0?A=60:A=49),g?b:A}else{let n=this.tone(e);if(this.background==null)return n;let a=this.background(e).getTone(e),s=this.contrastCurve.get(e.contrastLevel);if(z.ratioOfTones(a,n)>=s||(n=t.foregroundTone(a,s)),r&&(n=t.foregroundTone(a,s)),this.isBackground&&50<=n&&n<60&&(z.ratioOfTones(49,a)>=s?n=49:n=60),this.secondBackground){let[i,c]=[this.background,this.secondBackground],[h,m]=[i(e).getTone(e),c(e).getTone(e)],[u,y]=[Math.max(h,m),Math.min(h,m)];if(z.ratioOfTones(u,n)>=s&&z.ratioOfTones(y,n)>=s)return n;let p=z.lighter(u,s),f=z.darker(y,s),g=[];return p!==-1&&g.push(p),f!==-1&&g.push(f),t.tonePrefersLightForeground(h)||t.tonePrefersLightForeground(m)?p<0?100:p:g.length===1?g[0]:f<0?0:f}return n}}static foregroundTone(e,r){let n=z.lighterUnsafe(e,r),a=z.darkerUnsafe(e,r),s=z.ratioOfTones(n,e),i=z.ratioOfTones(a,e);if(t.tonePrefersLightForeground(e)){let h=Math.abs(s-i)<.1&&s<r&&i<r;return s>=r||s>=i||h?n:a}else return i>=r||i>=s?a:n}static tonePrefersLightForeground(e){return Math.round(e)<60}static toneAllowsLightForeground(e){return Math.round(e)<=49}static enableLightForeground(e){return t.tonePrefersLightForeground(e)&&!t.toneAllowsLightForeground(e)?49:e}};/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var P=class t{static fromInt(e){let r=I.fromInt(e);return t.fromHct(r)}static fromHct(e){return new t(e.hue,e.chroma,e)}static fromHueAndChroma(e,r){let n=new Ft(e,r).create();return new t(e,r,n)}constructor(e,r,n){this.hue=e,this.chroma=r,this.keyColor=n,this.cache=new Map}tone(e){let r=this.cache.get(e);return r===void 0&&(r=I.from(this.hue,this.chroma,e).toInt(),this.cache.set(e,r)),r}getHct(e){return I.fromInt(this.tone(e))}},Ft=class{constructor(e,r){this.hue=e,this.requestedChroma=r,this.chromaCache=new Map,this.maxChromaValue=200}create(){let a=0,s=100;for(;a<s;){let i=Math.floor((a+s)/2),c=this.maxChroma(i)<this.maxChroma(i+1);if(this.maxChroma(i)>=this.requestedChroma-.01)if(Math.abs(a-50)<Math.abs(s-50))s=i;else{if(a===i)return I.from(this.hue,this.requestedChroma,a);a=i}else c?a=i+1:s=i}return I.from(this.hue,this.requestedChroma,a)}maxChroma(e){if(this.chromaCache.has(e))return this.chromaCache.get(e);let r=I.from(this.hue,this.maxChromaValue,e).chroma;return this.chromaCache.set(e,r),r}};/**
 * @license
 * Copyright 2023 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var C=class{constructor(e,r,n,a){this.low=e,this.normal=r,this.medium=n,this.high=a}get(e){return e<=-1?this.low:e<0?Z(this.low,this.normal,(e- -1)/1):e<.5?Z(this.normal,this.medium,(e-0)/.5):e<1?Z(this.medium,this.high,(e-.5)/.5):this.high}};/**
 * @license
 * Copyright 2023 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var E=class{constructor(e,r,n,a,s){this.roleA=e,this.roleB=r,this.delta=n,this.polarity=a,this.stayTogether=s}};/**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var F;(function(t){t[t.MONOCHROME=0]="MONOCHROME",t[t.NEUTRAL=1]="NEUTRAL",t[t.TONAL_SPOT=2]="TONAL_SPOT",t[t.VIBRANT=3]="VIBRANT",t[t.EXPRESSIVE=4]="EXPRESSIVE",t[t.FIDELITY=5]="FIDELITY",t[t.CONTENT=6]="CONTENT",t[t.RAINBOW=7]="RAINBOW",t[t.FRUIT_SALAD=8]="FRUIT_SALAD"})(F||(F={}));/**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */function nt(t){return t.variant===F.FIDELITY||t.variant===F.CONTENT}function T(t){return t.variant===F.MONOCHROME}function ne(t,e,r,n){let a=r,s=I.from(t,e,r);if(s.chroma<e){let i=s.chroma;for(;s.chroma<e;){a+=n?-1:1;let c=I.from(t,e,a);if(i>c.chroma||Math.abs(c.chroma-e)<.4)break;let h=Math.abs(c.chroma-e),m=Math.abs(s.chroma-e);h<m&&(s=c),i=Math.max(i,c.chroma)}}return a}var o=class t{static highestSurface(e){return e.isDark?t.surfaceBright:t.surfaceDim}};o.contentAccentToneDelta=15;o.primaryPaletteKeyColor=d.fromPalette({name:"primary_palette_key_color",palette:t=>t.primaryPalette,tone:t=>t.primaryPalette.keyColor.tone});o.secondaryPaletteKeyColor=d.fromPalette({name:"secondary_palette_key_color",palette:t=>t.secondaryPalette,tone:t=>t.secondaryPalette.keyColor.tone});o.tertiaryPaletteKeyColor=d.fromPalette({name:"tertiary_palette_key_color",palette:t=>t.tertiaryPalette,tone:t=>t.tertiaryPalette.keyColor.tone});o.neutralPaletteKeyColor=d.fromPalette({name:"neutral_palette_key_color",palette:t=>t.neutralPalette,tone:t=>t.neutralPalette.keyColor.tone});o.neutralVariantPaletteKeyColor=d.fromPalette({name:"neutral_variant_palette_key_color",palette:t=>t.neutralVariantPalette,tone:t=>t.neutralVariantPalette.keyColor.tone});o.background=d.fromPalette({name:"background",palette:t=>t.neutralPalette,tone:t=>t.isDark?6:98,isBackground:!0});o.onBackground=d.fromPalette({name:"on_background",palette:t=>t.neutralPalette,tone:t=>t.isDark?90:10,background:t=>o.background,contrastCurve:new C(3,3,4.5,7)});o.surface=d.fromPalette({name:"surface",palette:t=>t.neutralPalette,tone:t=>t.isDark?6:98,isBackground:!0});o.surfaceDim=d.fromPalette({name:"surface_dim",palette:t=>t.neutralPalette,tone:t=>t.isDark?6:new C(87,87,80,75).get(t.contrastLevel),isBackground:!0});o.surfaceBright=d.fromPalette({name:"surface_bright",palette:t=>t.neutralPalette,tone:t=>t.isDark?new C(24,24,29,34).get(t.contrastLevel):98,isBackground:!0});o.surfaceContainerLowest=d.fromPalette({name:"surface_container_lowest",palette:t=>t.neutralPalette,tone:t=>t.isDark?new C(4,4,2,0).get(t.contrastLevel):100,isBackground:!0});o.surfaceContainerLow=d.fromPalette({name:"surface_container_low",palette:t=>t.neutralPalette,tone:t=>t.isDark?new C(10,10,11,12).get(t.contrastLevel):new C(96,96,96,95).get(t.contrastLevel),isBackground:!0});o.surfaceContainer=d.fromPalette({name:"surface_container",palette:t=>t.neutralPalette,tone:t=>t.isDark?new C(12,12,16,20).get(t.contrastLevel):new C(94,94,92,90).get(t.contrastLevel),isBackground:!0});o.surfaceContainerHigh=d.fromPalette({name:"surface_container_high",palette:t=>t.neutralPalette,tone:t=>t.isDark?new C(17,17,21,25).get(t.contrastLevel):new C(92,92,88,85).get(t.contrastLevel),isBackground:!0});o.surfaceContainerHighest=d.fromPalette({name:"surface_container_highest",palette:t=>t.neutralPalette,tone:t=>t.isDark?new C(22,22,26,30).get(t.contrastLevel):new C(90,90,84,80).get(t.contrastLevel),isBackground:!0});o.onSurface=d.fromPalette({name:"on_surface",palette:t=>t.neutralPalette,tone:t=>t.isDark?90:10,background:t=>o.highestSurface(t),contrastCurve:new C(4.5,7,11,21)});o.surfaceVariant=d.fromPalette({name:"surface_variant",palette:t=>t.neutralVariantPalette,tone:t=>t.isDark?30:90,isBackground:!0});o.onSurfaceVariant=d.fromPalette({name:"on_surface_variant",palette:t=>t.neutralVariantPalette,tone:t=>t.isDark?80:30,background:t=>o.highestSurface(t),contrastCurve:new C(3,4.5,7,11)});o.inverseSurface=d.fromPalette({name:"inverse_surface",palette:t=>t.neutralPalette,tone:t=>t.isDark?90:20});o.inverseOnSurface=d.fromPalette({name:"inverse_on_surface",palette:t=>t.neutralPalette,tone:t=>t.isDark?20:95,background:t=>o.inverseSurface,contrastCurve:new C(4.5,7,11,21)});o.outline=d.fromPalette({name:"outline",palette:t=>t.neutralVariantPalette,tone:t=>t.isDark?60:50,background:t=>o.highestSurface(t),contrastCurve:new C(1.5,3,4.5,7)});o.outlineVariant=d.fromPalette({name:"outline_variant",palette:t=>t.neutralVariantPalette,tone:t=>t.isDark?30:80,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5)});o.shadow=d.fromPalette({name:"shadow",palette:t=>t.neutralPalette,tone:t=>0});o.scrim=d.fromPalette({name:"scrim",palette:t=>t.neutralPalette,tone:t=>0});o.surfaceTint=d.fromPalette({name:"surface_tint",palette:t=>t.primaryPalette,tone:t=>t.isDark?80:40,isBackground:!0});o.primary=d.fromPalette({name:"primary",palette:t=>t.primaryPalette,tone:t=>T(t)?t.isDark?100:0:t.isDark?80:40,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(3,4.5,7,7),toneDeltaPair:t=>new E(o.primaryContainer,o.primary,10,"nearer",!1)});o.onPrimary=d.fromPalette({name:"on_primary",palette:t=>t.primaryPalette,tone:t=>T(t)?t.isDark?10:90:t.isDark?20:100,background:t=>o.primary,contrastCurve:new C(4.5,7,11,21)});o.primaryContainer=d.fromPalette({name:"primary_container",palette:t=>t.primaryPalette,tone:t=>nt(t)?t.sourceColorHct.tone:T(t)?t.isDark?85:25:t.isDark?30:90,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5),toneDeltaPair:t=>new E(o.primaryContainer,o.primary,10,"nearer",!1)});o.onPrimaryContainer=d.fromPalette({name:"on_primary_container",palette:t=>t.primaryPalette,tone:t=>nt(t)?d.foregroundTone(o.primaryContainer.tone(t),4.5):T(t)?t.isDark?0:100:t.isDark?90:30,background:t=>o.primaryContainer,contrastCurve:new C(3,4.5,7,11)});o.inversePrimary=d.fromPalette({name:"inverse_primary",palette:t=>t.primaryPalette,tone:t=>t.isDark?40:80,background:t=>o.inverseSurface,contrastCurve:new C(3,4.5,7,7)});o.secondary=d.fromPalette({name:"secondary",palette:t=>t.secondaryPalette,tone:t=>t.isDark?80:40,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(3,4.5,7,7),toneDeltaPair:t=>new E(o.secondaryContainer,o.secondary,10,"nearer",!1)});o.onSecondary=d.fromPalette({name:"on_secondary",palette:t=>t.secondaryPalette,tone:t=>T(t)?t.isDark?10:100:t.isDark?20:100,background:t=>o.secondary,contrastCurve:new C(4.5,7,11,21)});o.secondaryContainer=d.fromPalette({name:"secondary_container",palette:t=>t.secondaryPalette,tone:t=>{let e=t.isDark?30:90;return T(t)?t.isDark?30:85:nt(t)?ne(t.secondaryPalette.hue,t.secondaryPalette.chroma,e,!t.isDark):e},isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5),toneDeltaPair:t=>new E(o.secondaryContainer,o.secondary,10,"nearer",!1)});o.onSecondaryContainer=d.fromPalette({name:"on_secondary_container",palette:t=>t.secondaryPalette,tone:t=>T(t)?t.isDark?90:10:nt(t)?d.foregroundTone(o.secondaryContainer.tone(t),4.5):t.isDark?90:30,background:t=>o.secondaryContainer,contrastCurve:new C(3,4.5,7,11)});o.tertiary=d.fromPalette({name:"tertiary",palette:t=>t.tertiaryPalette,tone:t=>T(t)?t.isDark?90:25:t.isDark?80:40,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(3,4.5,7,7),toneDeltaPair:t=>new E(o.tertiaryContainer,o.tertiary,10,"nearer",!1)});o.onTertiary=d.fromPalette({name:"on_tertiary",palette:t=>t.tertiaryPalette,tone:t=>T(t)?t.isDark?10:90:t.isDark?20:100,background:t=>o.tertiary,contrastCurve:new C(4.5,7,11,21)});o.tertiaryContainer=d.fromPalette({name:"tertiary_container",palette:t=>t.tertiaryPalette,tone:t=>{if(T(t))return t.isDark?60:49;if(!nt(t))return t.isDark?30:90;let e=t.tertiaryPalette.getHct(t.sourceColorHct.tone);return tt.fixIfDisliked(e).tone},isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5),toneDeltaPair:t=>new E(o.tertiaryContainer,o.tertiary,10,"nearer",!1)});o.onTertiaryContainer=d.fromPalette({name:"on_tertiary_container",palette:t=>t.tertiaryPalette,tone:t=>T(t)?t.isDark?0:100:nt(t)?d.foregroundTone(o.tertiaryContainer.tone(t),4.5):t.isDark?90:30,background:t=>o.tertiaryContainer,contrastCurve:new C(3,4.5,7,11)});o.error=d.fromPalette({name:"error",palette:t=>t.errorPalette,tone:t=>t.isDark?80:40,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(3,4.5,7,7),toneDeltaPair:t=>new E(o.errorContainer,o.error,10,"nearer",!1)});o.onError=d.fromPalette({name:"on_error",palette:t=>t.errorPalette,tone:t=>t.isDark?20:100,background:t=>o.error,contrastCurve:new C(4.5,7,11,21)});o.errorContainer=d.fromPalette({name:"error_container",palette:t=>t.errorPalette,tone:t=>t.isDark?30:90,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5),toneDeltaPair:t=>new E(o.errorContainer,o.error,10,"nearer",!1)});o.onErrorContainer=d.fromPalette({name:"on_error_container",palette:t=>t.errorPalette,tone:t=>T(t)?t.isDark?90:10:t.isDark?90:30,background:t=>o.errorContainer,contrastCurve:new C(3,4.5,7,11)});o.primaryFixed=d.fromPalette({name:"primary_fixed",palette:t=>t.primaryPalette,tone:t=>T(t)?40:90,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5),toneDeltaPair:t=>new E(o.primaryFixed,o.primaryFixedDim,10,"lighter",!0)});o.primaryFixedDim=d.fromPalette({name:"primary_fixed_dim",palette:t=>t.primaryPalette,tone:t=>T(t)?30:80,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5),toneDeltaPair:t=>new E(o.primaryFixed,o.primaryFixedDim,10,"lighter",!0)});o.onPrimaryFixed=d.fromPalette({name:"on_primary_fixed",palette:t=>t.primaryPalette,tone:t=>T(t)?100:10,background:t=>o.primaryFixedDim,secondBackground:t=>o.primaryFixed,contrastCurve:new C(4.5,7,11,21)});o.onPrimaryFixedVariant=d.fromPalette({name:"on_primary_fixed_variant",palette:t=>t.primaryPalette,tone:t=>T(t)?90:30,background:t=>o.primaryFixedDim,secondBackground:t=>o.primaryFixed,contrastCurve:new C(3,4.5,7,11)});o.secondaryFixed=d.fromPalette({name:"secondary_fixed",palette:t=>t.secondaryPalette,tone:t=>T(t)?80:90,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5),toneDeltaPair:t=>new E(o.secondaryFixed,o.secondaryFixedDim,10,"lighter",!0)});o.secondaryFixedDim=d.fromPalette({name:"secondary_fixed_dim",palette:t=>t.secondaryPalette,tone:t=>T(t)?70:80,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5),toneDeltaPair:t=>new E(o.secondaryFixed,o.secondaryFixedDim,10,"lighter",!0)});o.onSecondaryFixed=d.fromPalette({name:"on_secondary_fixed",palette:t=>t.secondaryPalette,tone:t=>10,background:t=>o.secondaryFixedDim,secondBackground:t=>o.secondaryFixed,contrastCurve:new C(4.5,7,11,21)});o.onSecondaryFixedVariant=d.fromPalette({name:"on_secondary_fixed_variant",palette:t=>t.secondaryPalette,tone:t=>T(t)?25:30,background:t=>o.secondaryFixedDim,secondBackground:t=>o.secondaryFixed,contrastCurve:new C(3,4.5,7,11)});o.tertiaryFixed=d.fromPalette({name:"tertiary_fixed",palette:t=>t.tertiaryPalette,tone:t=>T(t)?40:90,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5),toneDeltaPair:t=>new E(o.tertiaryFixed,o.tertiaryFixedDim,10,"lighter",!0)});o.tertiaryFixedDim=d.fromPalette({name:"tertiary_fixed_dim",palette:t=>t.tertiaryPalette,tone:t=>T(t)?30:80,isBackground:!0,background:t=>o.highestSurface(t),contrastCurve:new C(1,1,3,4.5),toneDeltaPair:t=>new E(o.tertiaryFixed,o.tertiaryFixedDim,10,"lighter",!0)});o.onTertiaryFixed=d.fromPalette({name:"on_tertiary_fixed",palette:t=>t.tertiaryPalette,tone:t=>T(t)?100:10,background:t=>o.tertiaryFixedDim,secondBackground:t=>o.tertiaryFixed,contrastCurve:new C(4.5,7,11,21)});o.onTertiaryFixedVariant=d.fromPalette({name:"on_tertiary_fixed_variant",palette:t=>t.tertiaryPalette,tone:t=>T(t)?90:30,background:t=>o.tertiaryFixedDim,secondBackground:t=>o.tertiaryFixed,contrastCurve:new C(3,4.5,7,11)});/**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var B=class{constructor(e){this.sourceColorArgb=e.sourceColorArgb,this.variant=e.variant,this.contrastLevel=e.contrastLevel,this.isDark=e.isDark,this.sourceColorHct=I.fromInt(e.sourceColorArgb),this.primaryPalette=e.primaryPalette,this.secondaryPalette=e.secondaryPalette,this.tertiaryPalette=e.tertiaryPalette,this.neutralPalette=e.neutralPalette,this.neutralVariantPalette=e.neutralVariantPalette,this.errorPalette=P.fromHueAndChroma(25,84)}static getRotatedHue(e,r,n){let a=e.hue;if(r.length!==n.length)throw new Error(`mismatch between hue length ${r.length} & rotations ${n.length}`);if(n.length===1)return L(e.hue+n[0]);let s=r.length;for(let i=0;i<=s-2;i++){let c=r[i],h=r[i+1];if(c<a&&a<h)return L(a+n[i])}return a}getArgb(e){return e.getArgb(this)}getHct(e){return e.getHct(this)}get primaryPaletteKeyColor(){return this.getArgb(o.primaryPaletteKeyColor)}get secondaryPaletteKeyColor(){return this.getArgb(o.secondaryPaletteKeyColor)}get tertiaryPaletteKeyColor(){return this.getArgb(o.tertiaryPaletteKeyColor)}get neutralPaletteKeyColor(){return this.getArgb(o.neutralPaletteKeyColor)}get neutralVariantPaletteKeyColor(){return this.getArgb(o.neutralVariantPaletteKeyColor)}get background(){return this.getArgb(o.background)}get onBackground(){return this.getArgb(o.onBackground)}get surface(){return this.getArgb(o.surface)}get surfaceDim(){return this.getArgb(o.surfaceDim)}get surfaceBright(){return this.getArgb(o.surfaceBright)}get surfaceContainerLowest(){return this.getArgb(o.surfaceContainerLowest)}get surfaceContainerLow(){return this.getArgb(o.surfaceContainerLow)}get surfaceContainer(){return this.getArgb(o.surfaceContainer)}get surfaceContainerHigh(){return this.getArgb(o.surfaceContainerHigh)}get surfaceContainerHighest(){return this.getArgb(o.surfaceContainerHighest)}get onSurface(){return this.getArgb(o.onSurface)}get surfaceVariant(){return this.getArgb(o.surfaceVariant)}get onSurfaceVariant(){return this.getArgb(o.onSurfaceVariant)}get inverseSurface(){return this.getArgb(o.inverseSurface)}get inverseOnSurface(){return this.getArgb(o.inverseOnSurface)}get outline(){return this.getArgb(o.outline)}get outlineVariant(){return this.getArgb(o.outlineVariant)}get shadow(){return this.getArgb(o.shadow)}get scrim(){return this.getArgb(o.scrim)}get surfaceTint(){return this.getArgb(o.surfaceTint)}get primary(){return this.getArgb(o.primary)}get onPrimary(){return this.getArgb(o.onPrimary)}get primaryContainer(){return this.getArgb(o.primaryContainer)}get onPrimaryContainer(){return this.getArgb(o.onPrimaryContainer)}get inversePrimary(){return this.getArgb(o.inversePrimary)}get secondary(){return this.getArgb(o.secondary)}get onSecondary(){return this.getArgb(o.onSecondary)}get secondaryContainer(){return this.getArgb(o.secondaryContainer)}get onSecondaryContainer(){return this.getArgb(o.onSecondaryContainer)}get tertiary(){return this.getArgb(o.tertiary)}get onTertiary(){return this.getArgb(o.onTertiary)}get tertiaryContainer(){return this.getArgb(o.tertiaryContainer)}get onTertiaryContainer(){return this.getArgb(o.onTertiaryContainer)}get error(){return this.getArgb(o.error)}get onError(){return this.getArgb(o.onError)}get errorContainer(){return this.getArgb(o.errorContainer)}get onErrorContainer(){return this.getArgb(o.onErrorContainer)}get primaryFixed(){return this.getArgb(o.primaryFixed)}get primaryFixedDim(){return this.getArgb(o.primaryFixedDim)}get onPrimaryFixed(){return this.getArgb(o.onPrimaryFixed)}get onPrimaryFixedVariant(){return this.getArgb(o.onPrimaryFixedVariant)}get secondaryFixed(){return this.getArgb(o.secondaryFixed)}get secondaryFixedDim(){return this.getArgb(o.secondaryFixedDim)}get onSecondaryFixed(){return this.getArgb(o.onSecondaryFixed)}get onSecondaryFixedVariant(){return this.getArgb(o.onSecondaryFixedVariant)}get tertiaryFixed(){return this.getArgb(o.tertiaryFixed)}get tertiaryFixedDim(){return this.getArgb(o.tertiaryFixedDim)}get onTertiaryFixed(){return this.getArgb(o.onTertiaryFixed)}get onTertiaryFixedVariant(){return this.getArgb(o.onTertiaryFixedVariant)}};/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2023 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var it=class t{constructor(e){this.input=e,this.hctsByTempCache=[],this.hctsByHueCache=[],this.tempsByHctCache=new Map,this.inputRelativeTemperatureCache=-1,this.complementCache=null}get hctsByTemp(){if(this.hctsByTempCache.length>0)return this.hctsByTempCache;let e=this.hctsByHue.concat([this.input]),r=this.tempsByHct;return e.sort((n,a)=>r.get(n)-r.get(a)),this.hctsByTempCache=e,e}get warmest(){return this.hctsByTemp[this.hctsByTemp.length-1]}get coldest(){return this.hctsByTemp[0]}analogous(e=5,r=12){let n=Math.round(this.input.hue),a=this.hctsByHue[n],s=this.relativeTemperature(a),i=[a],c=0;for(let g=0;g<360;g++){let l=rt(n+g),x=this.hctsByHue[l],k=this.relativeTemperature(x),M=Math.abs(k-s);s=k,c+=M}let h=1,m=c/r,u=0;for(s=this.relativeTemperature(a);i.length<r;){let g=rt(n+h),l=this.hctsByHue[g],x=this.relativeTemperature(l),k=Math.abs(x-s);u+=k;let M=i.length*m,b=u>=M,R=1;for(;b&&i.length<r;){i.push(l);let A=(i.length+R)*m;b=u>=A,R++}if(s=x,h++,h>360){for(;i.length<r;)i.push(l);break}}let y=[this.input],p=Math.floor((e-1)/2);for(let g=1;g<p+1;g++){let l=0-g;for(;l<0;)l=i.length+l;l>=i.length&&(l=l%i.length),y.splice(0,0,i[l])}let f=e-p-1;for(let g=1;g<f+1;g++){let l=g;for(;l<0;)l=i.length+l;l>=i.length&&(l=l%i.length),y.push(i[l])}return y}get complement(){if(this.complementCache!=null)return this.complementCache;let e=this.coldest.hue,r=this.tempsByHct.get(this.coldest),n=this.warmest.hue,s=this.tempsByHct.get(this.warmest)-r,i=t.isBetween(this.input.hue,e,n),c=i?n:e,h=i?e:n,m=1,u=1e3,y=this.hctsByHue[Math.round(this.input.hue)],p=1-this.inputRelativeTemperature;for(let f=0;f<=360;f+=1){let g=L(c+m*f);if(!t.isBetween(g,c,h))continue;let l=this.hctsByHue[Math.round(g)],x=(this.tempsByHct.get(l)-r)/s,k=Math.abs(p-x);k<u&&(u=k,y=l)}return this.complementCache=y,this.complementCache}relativeTemperature(e){let r=this.tempsByHct.get(this.warmest)-this.tempsByHct.get(this.coldest),n=this.tempsByHct.get(e)-this.tempsByHct.get(this.coldest);return r===0?.5:n/r}get inputRelativeTemperature(){return this.inputRelativeTemperatureCache>=0?this.inputRelativeTemperatureCache:(this.inputRelativeTemperatureCache=this.relativeTemperature(this.input),this.inputRelativeTemperatureCache)}get tempsByHct(){if(this.tempsByHctCache.size>0)return this.tempsByHctCache;let e=this.hctsByHue.concat([this.input]),r=new Map;for(let n of e)r.set(n,t.rawTemperature(n));return this.tempsByHctCache=r,r}get hctsByHue(){if(this.hctsByHueCache.length>0)return this.hctsByHueCache;let e=[];for(let r=0;r<=360;r+=1){let n=I.from(r,this.input.chroma,this.input.tone);e.push(n)}return this.hctsByHueCache=e,this.hctsByHueCache}static isBetween(e,r,n){return r<n?r<=e&&e<=n:r<=e||e<=n}static rawTemperature(e){let r=St(e.toInt()),n=L(Math.atan2(r[2],r[1])*180/Math.PI),a=Math.sqrt(r[1]*r[1]+r[2]*r[2]);return-.5+.02*Math.pow(a,1.07)*Math.cos(L(n-50)*Math.PI/180)}};/**
 * @license
 * Copyright 2023 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var xt=class extends B{constructor(e,r,n){super({sourceColorArgb:e.toInt(),variant:F.CONTENT,contrastLevel:n,isDark:r,primaryPalette:P.fromHueAndChroma(e.hue,e.chroma),secondaryPalette:P.fromHueAndChroma(e.hue,Math.max(e.chroma-32,e.chroma*.5)),tertiaryPalette:P.fromInt(tt.fixIfDisliked(new it(e).analogous(3,6)[2]).toInt()),neutralPalette:P.fromHueAndChroma(e.hue,e.chroma/8),neutralVariantPalette:P.fromHueAndChroma(e.hue,e.chroma/8+4)})}};/**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var ct=class t extends B{constructor(e,r,n){super({sourceColorArgb:e.toInt(),variant:F.EXPRESSIVE,contrastLevel:n,isDark:r,primaryPalette:P.fromHueAndChroma(L(e.hue+240),40),secondaryPalette:P.fromHueAndChroma(B.getRotatedHue(e,t.hues,t.secondaryRotations),24),tertiaryPalette:P.fromHueAndChroma(B.getRotatedHue(e,t.hues,t.tertiaryRotations),32),neutralPalette:P.fromHueAndChroma(e.hue+15,8),neutralVariantPalette:P.fromHueAndChroma(e.hue+15,12)})}};ct.hues=[0,21,51,121,151,191,271,321,360];ct.secondaryRotations=[45,95,45,20,45,90,45,45,45];ct.tertiaryRotations=[120,120,20,45,20,15,20,120,120];/**
 * @license
 * Copyright 2023 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var Pt=class extends B{constructor(e,r,n){super({sourceColorArgb:e.toInt(),variant:F.MONOCHROME,contrastLevel:n,isDark:r,primaryPalette:P.fromHueAndChroma(e.hue,0),secondaryPalette:P.fromHueAndChroma(e.hue,0),tertiaryPalette:P.fromHueAndChroma(e.hue,0),neutralPalette:P.fromHueAndChroma(e.hue,0),neutralVariantPalette:P.fromHueAndChroma(e.hue,0)})}};/**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var bt=class extends B{constructor(e,r,n){super({sourceColorArgb:e.toInt(),variant:F.TONAL_SPOT,contrastLevel:n,isDark:r,primaryPalette:P.fromHueAndChroma(e.hue,36),secondaryPalette:P.fromHueAndChroma(e.hue,16),tertiaryPalette:P.fromHueAndChroma(L(e.hue+60),24),neutralPalette:P.fromHueAndChroma(e.hue,6),neutralVariantPalette:P.fromHueAndChroma(e.hue,8)})}};/**
 * @license
 * Copyright 2022 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var ht=class t extends B{constructor(e,r,n){super({sourceColorArgb:e.toInt(),variant:F.VIBRANT,contrastLevel:n,isDark:r,primaryPalette:P.fromHueAndChroma(e.hue,200),secondaryPalette:P.fromHueAndChroma(B.getRotatedHue(e,t.hues,t.secondaryRotations),24),tertiaryPalette:P.fromHueAndChroma(B.getRotatedHue(e,t.hues,t.tertiaryRotations),32),neutralPalette:P.fromHueAndChroma(e.hue,10),neutralVariantPalette:P.fromHueAndChroma(e.hue,12)})}};ht.hues=[0,41,61,101,131,181,251,301,360];ht.secondaryRotations=[18,15,10,12,15,18,15,12,12];ht.tertiaryRotations=[35,30,20,25,30,35,30,25,25];/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var oe={desired:4,fallbackColorARGB:4282549748,filter:!0};function ae(t,e){return t.score>e.score?-1:t.score<e.score?1:0}var K=class t{constructor(){}static score(e,r){let{desired:n,fallbackColorARGB:a,filter:s}={...oe,...r},i=[],c=new Array(360).fill(0),h=0;for(let[f,g]of e.entries()){let l=I.fromInt(f);i.push(l);let x=Math.floor(l.hue);c[x]+=g,h+=g}let m=new Array(360).fill(0);for(let f=0;f<360;f++){let g=c[f]/h;for(let l=f-14;l<f+16;l++){let x=rt(l);m[x]+=g}}let u=new Array;for(let f of i){let g=rt(Math.round(f.hue)),l=m[g];if(s&&(f.chroma<t.CUTOFF_CHROMA||l<=t.CUTOFF_EXCITED_PROPORTION))continue;let x=l*100*t.WEIGHT_PROPORTION,k=f.chroma<t.TARGET_CHROMA?t.WEIGHT_CHROMA_BELOW:t.WEIGHT_CHROMA_ABOVE,M=(f.chroma-t.TARGET_CHROMA)*k,b=x+M;u.push({hct:f,score:b})}u.sort(ae);let y=[];for(let f=90;f>=15;f--){y.length=0;for(let{hct:g}of u)if(y.find(x=>Tt(g.hue,x.hue)<f)||y.push(g),y.length>=n)break;if(y.length>=n)break}let p=[];y.length===0&&p.push(a);for(let f of y)p.push(f.toInt());return p}};K.TARGET_CHROMA=48;K.WEIGHT_PROPORTION=.7;K.WEIGHT_CHROMA_ABOVE=.3;K.WEIGHT_CHROMA_BELOW=.1;K.CUTOFF_CHROMA=5;K.CUTOFF_EXCITED_PROPORTION=.01;/**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *//**
 * @license
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */var Bt=(t,e,r)=>Math.max(e,Math.min(r,t)),se=t=>"#"+(t>>>0&16777215).toString(16).padStart(6,"0"),ie=(t,e)=>{let r=I.fromInt(t);return I.from(r.hue,r.chroma,e).toInt()},Rt=P.fromInt(4283215696),Ht=P.fromInt(4294940672);function ce(t){let e=(parseInt(t.seed.replace("#",""),16)|4278190080)>>>0,r=t.style,n=!!t.dark,a=I.fromInt(e),s=r==="soft"?new bt(a,n,0):r==="monochrome"?new Pt(a,n,0):new xt(a,n,0),i=r==="tinted"?Math.min(a.chroma/6,10):null,c=(p,f,g)=>{let l=p.palette(s),x=n&&g!==void 0?g:p.getTone(s);return I.from(l.hue,i??Math.min(l.chroma,f),x).toInt()},h={surface:c(o.surface,n?3:1.5),surfaceBright:c(o.surfaceBright,n?3:1.5),low:c(o.surfaceContainerLow,3),container:c(o.surfaceContainer,3,15),high:c(o.surfaceContainerHigh,3,21),highest:c(o.surfaceContainerHighest,3,27)};if(t.black&&n){let p=I.fromInt(h.surface).tone;for(let f of Object.keys(h))h[f]=ie(h[f],Bt(I.fromInt(h[f]).tone-p,0,100));h.surface=4278190080}let m=o.primary.getArgb(s),u=o.onPrimary.getArgb(s);if(r==="vivid"||r==="tinted"){let p=n?Bt(a.tone+3,50,75):Bt(a.tone,35,60);m=p===a.tone?e:I.from(a.hue,a.chroma,p).toInt(),u=p<=60?4294967295:P.fromHueAndChroma(a.hue,a.chroma).tone(10)}let y={page:n?h.surface:h.container,card:n?h.container:h.surfaceBright,block:n?h.high:h.container,inset:h.highest,chrome:n?h.low:h.highest,text:n?o.onSurface.palette(s).tone(97):o.onSurface.getArgb(s),muted:o.onSurfaceVariant.getArgb(s),accent:m,onAccent:u,secondaryContainer:o.secondaryContainer.getArgb(s),onSecondaryContainer:o.onSecondaryContainer.getArgb(s),success:Rt.tone(n?80:40),warning:Ht.tone(n?80:40),danger:o.error.getArgb(s),onDanger:o.onError.getArgb(s),line:c(o.outlineVariant,4),outline:o.outline.getArgb(s),successContainer:Rt.tone(n?30:90),onSuccessContainer:Rt.tone(n?90:30),warningContainer:Ht.tone(n?30:90),onWarningContainer:Ht.tone(n?90:30),dangerContainer:o.errorContainer.getArgb(s),onDangerContainer:o.onErrorContainer.getArgb(s)};return Object.fromEntries(Object.entries(y).map(([p,f])=>[p,se(f)]))}return Qt(he);})();
