// FlClash's seededColorScheme from lib/ui/color.dart and surface roles.
import {Hct, TonalPalette, SchemeContent, SchemeTonalSpot, SchemeMonochrome, MaterialDynamicColors as C} from '@material/material-color-utilities';
const clamp=(v,min,max)=>Math.max(min,Math.min(max,v));
const hex=v=>'#'+((v>>>0)&0xffffff).toString(16).padStart(6,'0');
const tone=(argb,t)=>{const c=Hct.fromInt(argb);return Hct.from(c.hue,c.chroma,t).toInt();};
const success=TonalPalette.fromInt(0xff4caf50),warning=TonalPalette.fromInt(0xffff9800);
export function palette(input){
 const seed=(parseInt(input.seed.replace('#',''),16)|0xff000000)>>>0,style=input.style,dark=!!input.dark;
 const source=Hct.fromInt(seed);
 const scheme=style==='soft'?new SchemeTonalSpot(source,dark,0):style==='monochrome'?new SchemeMonochrome(source,dark,0):new SchemeContent(source,dark,0);
 const tint=style==='tinted'?Math.min(source.chroma/6,10):null;
 const neutral=(role,chroma,darkTone)=>{
  const p=role.palette(scheme),t=(dark&&darkTone!==undefined)?darkTone:role.getTone(scheme);
  return Hct.from(p.hue,tint??Math.min(p.chroma,chroma),t).toInt();
 };
 const surfaces={
  surface:neutral(C.surface,dark?3:1.5),
  surfaceBright:neutral(C.surfaceBright,dark?3:1.5),
  low:neutral(C.surfaceContainerLow,3),
  container:neutral(C.surfaceContainer,3,15),
  high:neutral(C.surfaceContainerHigh,3,21),
  highest:neutral(C.surfaceContainerHighest,3,27),
 };
 if(input.black&&dark){
  const shift=Hct.fromInt(surfaces.surface).tone;
  for(const key of Object.keys(surfaces))surfaces[key]=tone(surfaces[key],clamp(Hct.fromInt(surfaces[key]).tone-shift,0,100));
  surfaces.surface=0xff000000;
 }
 let accent=C.primary.getArgb(scheme),onAccent=C.onPrimary.getArgb(scheme);
 if(style==='vivid'||style==='tinted'){
  const t=dark?clamp(source.tone+3,50,75):clamp(source.tone,35,60);
  accent=t===source.tone?seed:Hct.from(source.hue,source.chroma,t).toInt();
  onAccent=t<=60?0xffffffff:TonalPalette.fromHueAndChroma(source.hue,source.chroma).tone(10);
 }
 const colors={
  page:dark?surfaces.surface:surfaces.container,
  card:dark?surfaces.container:surfaces.surfaceBright,
  block:dark?surfaces.high:surfaces.container,
  inset:surfaces.highest,
  chrome:dark?surfaces.low:surfaces.highest,
  text:dark?C.onSurface.palette(scheme).tone(97):C.onSurface.getArgb(scheme),
  muted:C.onSurfaceVariant.getArgb(scheme),accent,onAccent,
  secondaryContainer:C.secondaryContainer.getArgb(scheme),onSecondaryContainer:C.onSecondaryContainer.getArgb(scheme),
  success:success.tone(dark?80:40),warning:warning.tone(dark?80:40),danger:C.error.getArgb(scheme),onDanger:C.onError.getArgb(scheme),
  line:neutral(C.outlineVariant,4),outline:C.outline.getArgb(scheme),
  successContainer:success.tone(dark?30:90),onSuccessContainer:success.tone(dark?90:30),
  warningContainer:warning.tone(dark?30:90),onWarningContainer:warning.tone(dark?90:30),
  dangerContainer:C.errorContainer.getArgb(scheme),onDangerContainer:C.onErrorContainer.getArgb(scheme),
 };
 return Object.fromEntries(Object.entries(colors).map(([k,v])=>[k,hex(v)]));
}
