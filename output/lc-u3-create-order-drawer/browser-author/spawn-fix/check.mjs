// Runs the actual driver's log/spawn setup using a disposable Node child instead of starting Next/browser/PG.
import assert from 'node:assert/strict';
import {readFile,mkdir} from 'node:fs/promises';
import {createWriteStream} from 'node:fs';
import {spawn} from 'node:child_process';
import {once} from 'node:events';
import path from 'node:path';
const source=await readFile('tests/admin/create-order-drawer-gate.mjs','utf8');
const logLine=source.match(/^const log=createWriteStream.*$/m)?.[0];
const awaitOpen=source.match(/^await once\(log,"open"\);$/m)?.[0]||'';
const spawnLine=source.match(/^ child=spawn.*$/m)?.[0];
assert(logLine&&spawnLine);
const code=logLine+'\n'+awaitOpen+'\nlet child;\nconst env={};\n'+spawnLine.replace('[path.join(root,"apps/admin/.next/standalone/apps/admin/server.js")]','["-e","process.exit(0)"]')+'\nawait once(child,"exit");\nawait new Promise(resolve=>log.end(resolve));';
const runDir=path.resolve('output/lc-u3-create-order-drawer/browser/spawn-fix',process.argv[2]||'check');
await mkdir(runDir,{recursive:true});
const AsyncFunction=Object.getPrototypeOf(async()=>{}).constructor;
try {
 await new AsyncFunction('createWriteStream','spawn','once','process','path','evidence','root',code)(createWriteStream,spawn,once,process,path,runDir,process.cwd());
 console.log('PASS actual driver spawn setup opens owned log before child launch');
} catch(error) {
 console.log('FAIL actual driver spawn setup',error.code||error.name);
 process.exitCode=1;
}
