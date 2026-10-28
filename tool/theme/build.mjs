import {build} from 'esbuild';
import {fileURLToPath} from 'node:url';
import {dirname,resolve} from 'node:path';
const here=dirname(fileURLToPath(import.meta.url));
await build({entryPoints:[resolve(here,'../../internal/theme/source/palette.js')],bundle:true,format:'iife',globalName:'FlClashPalette',nodePaths:[resolve(here,'node_modules')],outfile:resolve(here,'../../internal/theme/palette.js'),minify:true,legalComments:'inline',target:'es2023'});
