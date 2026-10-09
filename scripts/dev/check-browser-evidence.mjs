// Purpose: reject browser evidence writes outside the ignored per-run output tree.
// Depends on: Git tracked paths, TypeScript syntax parsing, and explicit filesystem write sinks.
// Used by: check-gates.sh and browser-evidence-paths.test.mjs; historical read-only inputs are allowed.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import ts from 'typescript-api';

const writers = new Map([
 ...['writeFile','writeFileSync','appendFile','appendFileSync','createWriteStream','mkdir','mkdirSync','mkdtemp','mkdtempSync','WriteFile','MkdirAll','Mkdir','MkdirTemp','Create','CreateTemp','CopyFS'].map(x=>[x,0]),
 ...['copyFile','copyFileSync','cp','cpSync','rename','renameSync','Rename'].map(x=>[x,1]),
]);
const constructors = new Set(['join','resolve','Join','Abs','Clean']);
const body = n => ts.isParenthesizedExpression(n) || ts.isAsExpression(n) || ts.isNonNullExpression(n) ? body(n.expression) : n;
const nameOf = n => ts.isPropertyAccessExpression(n) ? n.name.text : ts.isIdentifier(n) ? n.text : '';
const products = arrays => arrays.reduce((rows,values)=>rows.flatMap(row=>values.map(value=>[...row,value])),[[]]).slice(0,128);
function expression(text) {return ts.createSourceFile('expression.ts',`(${text})`,ts.ScriptTarget.Latest,true).statements[0]?.expression;}
/** Scan a producer's actual write destinations, following local path aliases and helper returns. */
export function inspectEvidenceSource(file,source,{tracked=[]}={}) {
 const findings=[], bindings=new Map(), returns=new Map();
 const add=(name,value)=>bindings.set(name,[...(bindings.get(name)||[]),value]);
 const tree=ts.createSourceFile(file,source,ts.ScriptTarget.Latest,true,file.endsWith('.tsx')?ts.ScriptKind.TSX:ts.ScriptKind.TS);
 function visit(n,callback){callback(n);ts.forEachChild(n,c=>visit(c,callback));}
 if(!file.endsWith('.go')) visit(tree,n=>{
  if(ts.isVariableDeclaration(n)&&ts.isIdentifier(n.name)&&n.initializer)add(n.name.text,n.initializer);
  if(ts.isBinaryExpression(n)&&n.operatorToken.kind===ts.SyntaxKind.EqualsToken&&ts.isIdentifier(n.left))add(n.left.text,n.right);
  if(ts.isFunctionDeclaration(n)&&n.name&&n.body){const values=[];visit(n.body,c=>{if(ts.isReturnStatement(c)&&c.expression)values.push(c.expression);});returns.set(n.name.text,values);}
 });
 function evaluate(input,seen=new Set()) {
  if(!input)return[];const n=body(input);
  if(ts.isStringLiteralLike(n))return[n.text];
  if(ts.isIdentifier(n)){
   if(seen.has(n.text))return[];
   return (bindings.get(n.text)||[]).flatMap(v=>evaluate(v,new Set([...seen,n.text])));
  }
  if(ts.isConditionalExpression(n))return [...evaluate(n.whenTrue,seen),...evaluate(n.whenFalse,seen)];
  if(ts.isBinaryExpression(n)){
   const a=evaluate(n.left,seen),b=evaluate(n.right,seen);
   if(n.operatorToken.kind===ts.SyntaxKind.PlusToken)return products([a.length?a:['*'],b.length?b:['*']]).map(v=>v.join(''));
   if([ts.SyntaxKind.BarBarToken,ts.SyntaxKind.QuestionQuestionToken].includes(n.operatorToken.kind))return [...a,...b];
  }
  if(ts.isTemplateExpression(n)){
   let values=[n.head.text];
   for(const span of n.templateSpans){const part=evaluate(span.expression,seen);values=products([values,part.length?part:['*']]).map(v=>v.join('')+span.literal.text);}
   return values;
  }
  if(ts.isCallExpression(n)){
   const name=nameOf(n.expression);
   if(name==='TempDir')return['__TEST_TEMP__'];
   if(constructors.has(name))return products(n.arguments.map(a=>{const v=evaluate(a,seen);return v.length?v:['*'];})).map(v=>path.posix.normalize(v.join('/')));
   if(['mkdtemp','mkdtempSync','MkdirTemp','CreateTemp'].includes(name))return evaluate(n.arguments[0],seen);
   if(!seen.has(name)&&returns.has(name))return returns.get(name).flatMap(v=>evaluate(v,new Set([...seen,name])));
  }
  return[];
 }
 function unsafe(value){
  const normalized=path.posix.normalize(value.replaceAll('\\','/'));
  if(normalized==='__TEST_TEMP__'||normalized.startsWith('__TEST_TEMP__/'))return false;
  const match=normalized.match(/(?:^|\/)output(?:\/|$)/);
  if(match){const relative=normalized.slice(match.index+(normalized[match.index]==='/'?1:0));return relative!=='output/playwright'&&!relative.startsWith('output/playwright/');}
  // Explicit golden-baseline maintenance is not a browser gate. Keep its intentional opt-in command.
  if(file==='tests/admin/shell-runner.mjs'&&normalized.startsWith('tests/admin/baselines/w0')&&source.includes('--baseline'))return false;
  return tracked.filter(p=>p.startsWith('.impeccable/')).some(p=>normalized===p||normalized.endsWith('/'+p));
 }
 function check(n,position){
  for(const value of new Set(evaluate(n)))if(unsafe(value)){
   const line=source.slice(0,position).split('\n').length;
   if(!findings.some(f=>f.line===line&&f.path===value))findings.push({file,line,path:value});
  }
 }
 if(!file.endsWith('.go'))visit(tree,n=>{
  if(ts.isCallExpression(n)||ts.isNewExpression(n)){
   const name=nameOf(n.expression),args=n.arguments||[];
   if(writers.has(name))check(args[writers.get(name)],n.getStart(tree));
   if(['screenshot','pdf'].includes(name)&&args[0]){
    const options=body(args[0]);
    if(ts.isObjectLiteralExpression(options))for(const p of options.properties)if(ts.isPropertyAssignment(p)&&nameOf(p.name)==='path')check(p.initializer,n.getStart(tree));
   }
  }
  if(ts.isPropertyAssignment(n)&&['outputDir','outputFile'].includes(nameOf(n.name)))check(n.initializer,n.getStart(tree));
 });
 else {
  // Go's path expressions share the small literal/Join grammar above. Mask comments and
  // string contents before locating calls/assignments so SQL and generated config text are not code.
  let masked='',i=0;
  while(i<source.length){
   const start=i,c=source[i];
   if(source.startsWith('//',i)){while(i<source.length&&source[i]!=='\n')i++;}
   else if(source.startsWith('/*',i)){i=source.indexOf('*/',i+2);i=i<0?source.length:i+2;}
   else if(c==='"'||c==='\x60'||c==="'"){i++;while(i<source.length){if(source[i]===c){i++;break;}if(source[i]==='\\'&&c!=='\x60')i++;i++;}}
   else{masked+=c;i++;continue;}
   masked+=source.slice(start,i).replace(/[^\n]/g,' ');
  }
  function end(start){let depth=0;for(let j=start;j<masked.length;j++){const c=masked[j];if('([{'.includes(c))depth++;if(')]}'.includes(c)){if(!depth)return j;depth--;}if(!depth&&['\n',';'].includes(c))return j;}return masked.length;}
  for(const m of masked.matchAll(/\b(?:var\s+|const\s+)?([A-Za-z_]\w*)\s*(?::=|=(?!=))\s*/g)){const start=m.index+m[0].length;const n=expression(source.slice(start,end(start)));if(n)add(m[1],n);}
  for(const m of masked.matchAll(/\b(?:[A-Za-z_]\w*\.)?([A-Za-z_]\w*)\s*\(/g)){
   if(!writers.has(m[1]))continue;
   const start=m.index,open=start+m[0].lastIndexOf('(');let depth=1,j=open+1;
   for(;j<masked.length&&depth;j++){if(masked[j]==='(')depth++;if(masked[j]===')')depth--;}
   const n=body(expression(source.slice(start,j)));
   if(n&&ts.isCallExpression(n))check(n.arguments[writers.get(m[1])],start);
  }
 }
 return findings;
}
/** The ratchet covers browser producers, not documentation, historical artifacts or reader-only aggregators. */
export function browserProducer(file){
 return /^tests\/foundation\/browser[^/]*_test\.go$/.test(file)||file==='playwright.config.ts'||file==='tests/browser-evidence.mjs'||
  (/^tests\/admin\//.test(file)&&(/\.(spec|acceptance)\.tsx?$/.test(file)||/(-runner|-browser)\.mjs$/.test(file)))||
  (/^tests\/storefront\/.*\.mjs$/.test(file)&&!file.endsWith('.test.mjs'))||
  /^tests\/ui\/(click-sweep|visual-audit)(-lib)?\.mjs$/.test(file)||file==='tests/deploy/platform-edge.mjs';
}
/** Audit the tracked producer source and tracked output history without running a browser or writing files. */
export function checkBrowserEvidence(root=process.cwd()){
 const tracked=execFileSync('git',['ls-files','-z'],{cwd:root,encoding:'utf8'}).split('\0').filter(Boolean);
 return tracked.filter(browserProducer).flatMap(file=>inspectEvidenceSource(file,fs.readFileSync(path.join(root,file),'utf8'),{tracked}));
}
if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url)){
 const errors=checkBrowserEvidence();
 for(const e of errors)console.error(`browser-evidence: ${e.file}:${e.line}: unsafe write ${e.path}; use the run evidence directory under output/playwright`);
 console.log(`browser-evidence: ${errors.length?'FAIL':'PASS'} (${errors.length} unsafe write destinations)`);
 process.exitCode=errors.length?1:0;
}
