// Purpose: bound one consuming-build client-JS catalog, exact public-location lookup and inert byte retention.
// Depends on: Node fs/path/crypto; Next's per-segment encodeURIComponent static URL contract.
// Used by: shop-real-gate's opt-in observer and registered c7-rsc-pageerror tests; never fetches or executes assets.
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';

const CAPS = Object.freeze({ entries: 4096, files: 512, fileBytes: 8*1024*1024, totalBytes: 64*1024*1024, pathBytes: 1024, depth: 16, retainedBytes: 16*1024*1024, matches: 128, stackFrameMatches: 384, stackBytes: 16384, stackLines: 32, stackFrames: 3 });
const hash = b => createHash('sha256').update(b).digest('hex');
const bad = code => { throw new Error(code); };
const UNKNOWN = reason => ({ status:'UNKNOWN', reason, asset_id:'UNKNOWN', content_sha256:'UNKNOWN', catalog_epoch:'UNKNOWN', catalog_manifest_sha256:'UNKNOWN', byte_length:'UNKNOWN', evidence_path:'UNKNOWN', coordinate_units:'UNKNOWN', source_frame:'UNKNOWN', browser_response_bytes:'UNKNOWN' });
const UNKNOWN_FRAME = (ordinal, reason, status = 'UNKNOWN') => ({ frame_ordinal: ordinal, status, reason, asset_id:'UNKNOWN', content_sha256:'UNKNOWN', catalog_epoch:'UNKNOWN', catalog_manifest_sha256:'UNKNOWN', byte_length:'UNKNOWN', evidence_path:'UNKNOWN', raw_line:'UNKNOWN', raw_column:'UNKNOWN', coordinate_basis:'UNKNOWN', coordinate_units:'UNKNOWN', coordinate_representation:'UNKNOWN', coordinate_range:'UNKNOWN', zero_based_in_range:'UNKNOWN', one_based_in_range:'UNKNOWN', raw_basis_ambiguity:'UNKNOWN', zero_coordinate_ambiguity:'UNKNOWN', candidate_role:'UNKNOWN', source_frame:'UNKNOWN', browser_response_bytes:'UNKNOWN' });
const UNKNOWN_STACK = (reason, status = 'UNKNOWN', frames = [], truncated = 'UNKNOWN') => ({ status, reason, frame_count: frames.length, frames_truncated: truncated, parent_frame_ordinal:'UNKNOWN', parent_asset_id:'UNKNOWN', parent_content_sha256:'UNKNOWN', parent_raw_line:'UNKNOWN', parent_raw_column:'UNKNOWN', coordinate_basis:'UNKNOWN', coordinate_units:'UNKNOWN', coordinate_representation:'UNKNOWN', source_frame:'UNKNOWN', caller_identity:'UNKNOWN', request_causality:'UNKNOWN', frames });
const token = s => [s.dev,s.ino,s.size,s.mtimeNs,s.ctimeNs].join(':');
function parseWebKitFrameLine(line) {
  if(!line.startsWith('    at ')||!line.endsWith(')'))return null;
  const body=line.slice(7,-1), sep=body.indexOf(' ('); if(sep<=0)return null;
  const fn=body.slice(0,sep), loc=body.slice(sep+2);
  if(!fn||fn.includes('(')||fn.includes(')')||loc.includes('(')||loc.includes(')'))return null;
  if(loc==='native'||loc==='[native code]')return {kind:'NATIVE'};
  if(/\s/.test(loc)&&!loc.startsWith('[native code]:'))return null;
  const c2=loc.lastIndexOf(':'); if(c2<0)return null;
  const c1=loc.lastIndexOf(':',c2-1); if(c1<0)return null;
  const url=loc.slice(0,c1), lineStr=loc.slice(c1+1,c2), colStr=loc.slice(c2+1);
  if(!lineStr||!colStr||lineStr.includes('/')||colStr.includes('/'))return null;
  if(!/^(0|[1-9]\d*)$/.test(lineStr)||!/^(0|[1-9]\d*)$/.test(colStr))return {kind:'BAD_COORDINATES'};
  const lineNum=Number(lineStr), colNum=Number(colStr);
  if(!Number.isSafeInteger(lineNum)||!Number.isSafeInteger(colNum))return {kind:'BAD_COORDINATES'};
  if(url==='native'||url==='[native code]')return {kind:'NATIVE'};
  if(url==='')return {kind:'EMPTY_URL'};
  return {kind:'FRAME',url,line:lineNum,column:colNum};
}
function canonicalStaticPath(pathname) {
  if(!pathname.startsWith('/_next/static/'))return false;
  const parts=pathname.slice('/_next/static/'.length).split('/');
  if(!parts.length)return false;
  for(const seg of parts){
    if(!seg||seg==='.'||seg==='..')return false;
    let dec;try{dec=decodeURIComponent(seg);}catch{return false;}
    if(!dec||dec==='.'||dec==='..'||dec.includes('/')||dec.includes('\\')||dec.includes('\0')||encodeURIComponent(dec)!==seg)return false;
  }
  return true;
}

export function createClientAssetCatalog({ appRoot, evidenceRoot, origin, limits = {} }) {
  let state='COMPLETE', reason='NONE', initial, root, out, ownedDir, ownedToken, manifestHash=null, retainedBytes=0, calls=0, stackCalls=0, serial=0;
  const retained=new Map(), records=new Map(), stackRecords=new Map(), seenStacks=new Set(), caps={...CAPS};
  const fail = code => { state='UNKNOWN'; reason=code; return UNKNOWN(code); };
  const guard = fn => { try { return fn(); } catch(e) { return fail(['LIMIT','PATH','NONREGULAR','SYMLINK','CHANGED','BUILD_ID','DUPLICATE','DESTINATION','RETENTION','MANIFEST','COORDINATES','ENCODING'].includes(e?.message)?e.message:'IO'); } };
  function dirs(anchor, target) {
    const rel=path.relative(anchor,target); if(rel.startsWith('..')||path.isAbsolute(rel))bad('PATH');
    let current=anchor;
    for(const part of ['',...rel.split(path.sep).filter(Boolean)]) { current=part?path.join(current,part):current; const s=fs.lstatSync(current); if(s.isSymbolicLink())bad('SYMLINK'); if(!s.isDirectory())bad('NONREGULAR'); }
  }
  function read(file, maximum, anchor) {
    dirs(anchor,path.dirname(file)); const leaf=fs.lstatSync(file); if(leaf.isSymbolicLink())bad('SYMLINK'); if(!leaf.isFile())bad('NONREGULAR');
    const fd=fs.openSync(file,fs.constants.O_RDONLY|fs.constants.O_NOFOLLOW);
    try {
      const a=fs.fstatSync(fd,{bigint:true}); if(!a.isFile())bad('NONREGULAR'); if(a.size>BigInt(maximum))bad('LIMIT');
      const size=Number(a.size), buffer=Buffer.alloc(size+1); let n=0;
      while(n<buffer.length) { const got=fs.readSync(fd,buffer,n,buffer.length-n,null); if(!got)break; n+=got; }
      const b=fs.fstatSync(fd,{bigint:true}); if(token(a)!==token(b)||n!==size)bad('CHANGED');
      dirs(anchor,path.dirname(file)); if(fs.realpathSync(file)!==file||token(fs.lstatSync(file,{bigint:true}))!==token(a))bad('CHANGED');
      return {bytes:buffer.subarray(0,size),token:token(a)};
    } finally { fs.closeSync(fd); }
  }
  function scan() {
    dirs(root,path.join(root,'.next','static'));
    const build=read(path.join(root,'.next','BUILD_ID'),4096,root); if(!build.bytes.length)bad('BUILD_ID');
    const entries=[], tree=[]; let seen=0,total=0;
    function walk(dir,segments) {
      if(segments.length>caps.depth)bad('LIMIT'); dirs(root,dir);
      const handle=fs.opendirSync(dir,{encoding:'buffer'});
      try { let entry; while((entry=handle.readSync())) {
        if(++seen>caps.entries)bad('LIMIT');
        const raw=Buffer.from(entry.name), name=raw.toString('utf8');
        if(!Buffer.from(name).equals(raw)||!name||name.includes('\\')||name.includes('\0')||name==='.'||name==='..')bad('PATH');
        const parts=[...segments,name], rel=parts.join('/'); if(Buffer.byteLength(rel)>caps.pathBytes)bad('LIMIT');
        const file=path.join(dir,name), s=fs.lstatSync(file,{bigint:true}); if(s.isSymbolicLink())bad('SYMLINK');
        if(!s.isDirectory()&&!s.isFile())bad('NONREGULAR'); tree.push([rel,token(s)]);
        if(s.isDirectory()) { walk(file,parts); continue; }
        if(!name.endsWith('.js'))continue;
        if(entries.length>=caps.files)bad('LIMIT'); const data=read(file,caps.fileBytes,root); total+=data.bytes.length;if(total>caps.totalBytes)bad('LIMIT');
        const canonical='/_next/static/'+parts.map(encodeURIComponent).join('/');if(Buffer.byteLength(canonical)>caps.pathBytes)bad('LIMIT');
        entries.push({canonical,file,bytes:data.bytes,sha:hash(data.bytes),asset_id:'asset-'+hash(canonical)});
      }} finally { handle.closeSync(); }
    }
    walk(path.join(root,'.next','static'),[]); entries.sort((a,b)=>a.canonical.localeCompare(b.canonical));
    if(new Set(entries.map(x=>x.canonical)).size!==entries.length)bad('DUPLICATE');
    const manifest={version:1,scope:'regular-client-js-only',build_id_sha256:hash(build.bytes),entries:entries.map(e=>({asset_id:e.asset_id,content_sha256:e.sha,byte_length:e.bytes.length}))};
    const catalogHash=hash(JSON.stringify(manifest)), epoch=hash(JSON.stringify({build_id_sha256:manifest.build_id_sha256,catalog_manifest_sha256:catalogHash}));
    return {entries,manifest,catalogHash,epoch,signature:hash(JSON.stringify({tree:tree.sort(),build:build.token,epoch})),total};
  }
  function verify() { if(state!=='COMPLETE')bad(reason); const current=scan(); if(current.signature!==initial.signature)bad('CHANGED'); }
  function destination() {
    const s=fs.lstatSync(evidenceRoot); if(s.isSymbolicLink()||!s.isDirectory())bad('DESTINATION');
    out=fs.realpathSync(evidenceRoot);if(out!==path.resolve(evidenceRoot))bad('DESTINATION');dirs(out,out);
    const target=path.join(out,'c7-client-assets');
    if(!ownedDir) { fs.mkdirSync(target,{mode:0o700}); ownedDir=target; ownedToken=token(fs.lstatSync(target,{bigint:true})).split(':').slice(0,2).join(':'); }
    const d=fs.lstatSync(ownedDir,{bigint:true}); if(d.isSymbolicLink()||!d.isDirectory()||[d.dev,d.ino].join(':')!==ownedToken||fs.realpathSync(ownedDir)!==ownedDir)bad('DESTINATION');
    return ownedDir;
  }
  function persist(sealed=false) {
    if(!ownedDir)return;
    const dir=destination(), target=path.join(dir,'manifest.json');
    if(manifestHash!==null&&hash(read(target,256*1024,out).bytes)!==manifestHash)bad('MANIFEST');
    if(manifestHash===null&&fs.existsSync(target))bad('DESTINATION');
    const object={status:state,reason,sealed, catalog_epoch:initial?.epoch??'UNKNOWN',catalog_manifest_sha256:initial?.catalogHash??'UNKNOWN',catalog:initial?.manifest??null,matches:[...records.values()].map(x=>state==='COMPLETE'?x:{error_id:x.error_id,...UNKNOWN(reason)}),stack_frame_matches:[...stackRecords.values()].map(x=>state==='COMPLETE'?x:{error_id:x.error_id,...UNKNOWN_FRAME(x.frame_ordinal,reason)})};
    const bytes=Buffer.from(JSON.stringify(object,null,2)); if(bytes.length>256*1024)bad('LIMIT');
    const temp=path.join(dir,'.manifest-'+(++serial)+'.tmp'); let made=false;
    try { const fd=fs.openSync(temp,fs.constants.O_WRONLY|fs.constants.O_CREAT|fs.constants.O_EXCL|fs.constants.O_NOFOLLOW,0o600); made=true; try { fs.writeFileSync(fd,bytes); } finally {fs.closeSync(fd);} destination(); fs.renameSync(temp,target); made=false; manifestHash=hash(bytes); if(hash(read(target,256*1024,out).bytes)!==manifestHash)bad('MANIFEST'); }
    finally {if(made)fs.unlinkSync(temp);}
  }
  guard(()=>{
    for(const [k,v] of Object.entries(limits)) { if(!(k in CAPS)||!Number.isSafeInteger(v)||v<=0||v>CAPS[k])bad('LIMIT');caps[k]=v; }
    const a=fs.lstatSync(appRoot);if(a.isSymbolicLink()||!a.isDirectory())bad('PATH');root=fs.realpathSync(appRoot);if(root!==path.resolve(appRoot))bad('PATH');
    if(new URL(origin).origin!==origin)bad('PATH'); initial=scan();
  });
  const match = (url,line,column,errorID) => {
    if(state!=='COMPLETE')return UNKNOWN(reason);
    return guard(()=>{
      if(++calls>caps.matches)bad('LIMIT');verify();
      // Literal canonical membership rejects dot normalization, encoded aliases, credentials, query and fragment.
      if(typeof url!=='string')return UNKNOWN('URL'); let u;try{u=new URL(url);}catch{return UNKNOWN('URL');}
      if(u.origin!==origin||u.username||u.password||u.search||u.hash||url!==origin+u.pathname)return UNKNOWN('URL');
      const entry=initial.entries.find(e=>url===origin+e.canonical); if(!entry)return UNKNOWN('NOT_CATALOGED');
      if(!Number.isSafeInteger(errorID)||errorID<1||records.has(errorID))return UNKNOWN('ERROR_ID');
      if(!Number.isSafeInteger(line)||line<0||!Number.isSafeInteger(column)||column<0)return UNKNOWN('COORDINATES');
      let text;try{text=new TextDecoder('utf-8',{fatal:true,ignoreBOM:true}).decode(entry.bytes);}catch{bad('ENCODING');}
      const lines=text.split(/\r\n|\r|\n/);if(line>=lines.length||column>lines[line].length)return UNKNOWN('COORDINATES');
      const dir=destination(), file=path.join(dir,entry.sha+'.js');
      if(!retained.has(entry.sha)) {
        if(retainedBytes+entry.bytes.length>caps.retainedBytes)return UNKNOWN('RETENTION');
        if(fs.existsSync(file))bad('DESTINATION');
        const fd=fs.openSync(file,fs.constants.O_WRONLY|fs.constants.O_CREAT|fs.constants.O_EXCL|fs.constants.O_NOFOLLOW,0o600);try{fs.writeFileSync(fd,entry.bytes);}finally{fs.closeSync(fd);}
        retained.set(entry.sha,entry.bytes.length);retainedBytes+=entry.bytes.length;
      }
      if(hash(read(file,caps.fileBytes,out).bytes)!==entry.sha)bad('RETENTION');
      const result={status:'MATCHED',reason:'NONE',asset_id:entry.asset_id,content_sha256:entry.sha,catalog_epoch:initial.epoch,catalog_manifest_sha256:initial.catalogHash,byte_length:entry.bytes.length,evidence_path:'c7-client-assets/'+entry.sha+'.js',coordinate_units:'UNKNOWN',coordinate_representation:'utf8-decoded-utf16-code-units',coordinate_range:'IN_RANGE_REPRESENTATION',source_frame:'UNKNOWN',browser_response_bytes:'UNKNOWN'};
      records.set(errorID,{error_id:errorID,...result,public_line:line,public_column:column});persist(false);return result;
    });
  };
  const matchStack = (stack, errorID, headerSafe = true) => {
    if(state!=='COMPLETE')return UNKNOWN_STACK(reason);
    if(!Number.isSafeInteger(errorID)||errorID<1||seenStacks.has(errorID))return UNKNOWN_STACK('ERROR_ID');
    seenStacks.add(errorID);
    if(!headerSafe)return UNKNOWN_STACK('MALFORMED_STACK');
    if(typeof stack!=='string')return UNKNOWN_STACK('STACK_UNAVAILABLE');
    if(Buffer.byteLength(stack,'utf8')>caps.stackBytes)return UNKNOWN_STACK('STACK_BYTES_LIMIT');
    if(stack==='')return UNKNOWN_STACK('EMPTY_STACK','INSUFFICIENT',[],false);
    if(stack.includes('\0')||stack.includes('\r'))return UNKNOWN_STACK('MALFORMED_STACK');
    const rawLines=stack.split('\n');
    if(rawLines.length>caps.stackLines)return UNKNOWN_STACK('STACK_LINES_LIMIT');
    if(rawLines[0].startsWith('    at ')||/^\s*at\s/.test(rawLines[0]))return UNKNOWN_STACK('MALFORMED_STACK');
    if(rawLines.length===1||(rawLines.length===2&&rawLines[1]===''))return UNKNOWN_STACK('NO_FRAMES','INSUFFICIENT',[],false);
    const frameLines=rawLines.slice(1), parsed=[];
    for(const line of frameLines){const p=parseWebKitFrameLine(line);if(!p)return UNKNOWN_STACK('MALFORMED_STACK');parsed.push(p);}
    const truncated=parsed.length>caps.stackFrames, take=parsed.slice(0,caps.stackFrames), frames=[];
    let hardFailure=null, persisted=false;
    for(let i=0;i<take.length;i++) {
      const ordinal=i+1, item=take[i];
      const frame = guard(()=>{
        if(++stackCalls>caps.stackFrameMatches)bad('LIMIT');verify();
        if(item.kind==='NATIVE')return UNKNOWN_FRAME(ordinal,'NATIVE_FRAME');
        if(item.kind==='EMPTY_URL')return UNKNOWN_FRAME(ordinal,'EMPTY_URL');
        if(item.kind==='BAD_COORDINATES')return UNKNOWN_FRAME(ordinal,'COORDINATES');
        const {url,line,column}=item;
        if(line>caps.fileBytes||column>caps.fileBytes)return UNKNOWN_FRAME(ordinal,'COORDINATES');
        if(typeof url!=='string'||Buffer.byteLength(url,'utf8')>caps.pathBytes+origin.length)return UNKNOWN_FRAME(ordinal,'URL');
        let u;try{u=new URL(url);}catch{return UNKNOWN_FRAME(ordinal,'URL');}
        if(u.origin!==origin||u.username||u.password||u.search||u.hash||url!==origin+u.pathname||!canonicalStaticPath(u.pathname))return UNKNOWN_FRAME(ordinal,'URL');
        const entry=initial.entries.find(e=>url===origin+e.canonical);if(!entry)return UNKNOWN_FRAME(ordinal,'NOT_CATALOGED');
        let text;try{text=new TextDecoder('utf-8',{fatal:true,ignoreBOM:true}).decode(entry.bytes);}catch{bad('ENCODING');}
        const lines=text.split(/\r\n|\r|\n/);
        const inZero=line<lines.length&&column<=lines[line].length;
        const inOne=line>=1&&column>=1&&(line-1)<lines.length&&(column-1)<=lines[line-1].length;
        if(!inZero&&!inOne)return UNKNOWN_FRAME(ordinal,'COORDINATES');
        const dir=destination(), file=path.join(dir,entry.sha+'.js');
        if(!retained.has(entry.sha)) {
          if(retainedBytes+entry.bytes.length>caps.retainedBytes)return UNKNOWN_FRAME(ordinal,'RETENTION');
          if(fs.existsSync(file))bad('DESTINATION');
          const fd=fs.openSync(file,fs.constants.O_WRONLY|fs.constants.O_CREAT|fs.constants.O_EXCL|fs.constants.O_NOFOLLOW,0o600);try{fs.writeFileSync(fd,entry.bytes);}finally{fs.closeSync(fd);}
          retained.set(entry.sha,entry.bytes.length);retainedBytes+=entry.bytes.length;
        }
        if(hash(read(file,caps.fileBytes,out).bytes)!==entry.sha)bad('RETENTION');
        const repeated=ordinal>1&&frames.some(f=>f.status==='MATCHED'&&f.asset_id===entry.asset_id&&f.raw_line===line&&f.raw_column===column);
        const role=ordinal===1?'TOP_FRAME_SYNTAX_CANDIDATE':repeated?'REPEATED_FRAME_SYNTAX_CANDIDATE':'PARENT_SYNTAX_CANDIDATE';
        const res={frame_ordinal:ordinal,status:'MATCHED',reason:'NONE',asset_id:entry.asset_id,content_sha256:entry.sha,catalog_epoch:initial.epoch,catalog_manifest_sha256:initial.catalogHash,byte_length:entry.bytes.length,evidence_path:'c7-client-assets/'+entry.sha+'.js',raw_line:line,raw_column:column,coordinate_basis:'raw-callframe-stack',coordinate_units:'UNKNOWN',coordinate_representation:'utf8-decoded-utf16-code-units-representation-only',coordinate_range:'IN_RANGE_REPRESENTATION_ONLY',zero_based_in_range:inZero,one_based_in_range:inOne,raw_basis_ambiguity:true,zero_coordinate_ambiguity:line===0||column===0,candidate_role:role,source_frame:'UNKNOWN',browser_response_bytes:'UNKNOWN'};
        stackRecords.set(errorID+':'+ordinal,{error_id:errorID,...res});persisted=true;return res;
      });
      if(state!=='COMPLETE')return UNKNOWN_STACK(reason,'UNKNOWN',frames.map((_,idx)=>UNKNOWN_FRAME(idx+1,reason)));
      const normalized = frame.frame_ordinal ? frame : UNKNOWN_FRAME(ordinal, frame.reason ?? reason);
      frames.push(normalized);
      if(['URL','COORDINATES','RETENTION'].includes(normalized.reason))hardFailure??=normalized.reason;
    }
    if(persisted||ownedDir){const ok=guard(()=>{persist(false);return true;});if(ok!==true||state!=='COMPLETE')return UNKNOWN_STACK(reason,'UNKNOWN',frames.map((_,idx)=>UNKNOWN_FRAME(idx+1,reason)));}
    if(hardFailure)return UNKNOWN_STACK(hardFailure,'UNKNOWN',frames,truncated);
    const top=frames[0], parent=frames.find(f=>f.frame_ordinal>=2&&f.status==='MATCHED'&&f.candidate_role==='PARENT_SYNTAX_CANDIDATE');
    if(parent){
      const distinct=top?.status==='MATCHED'&&top.asset_id!==parent.asset_id;
      return {status:'MATCHED',reason:distinct?'DISTINCT_PARENT_ASSET':'PARENT_CANDIDATE_IDENTIFIED',frame_count:frames.length,frames_truncated:truncated,parent_frame_ordinal:parent.frame_ordinal,parent_asset_id:parent.asset_id,parent_content_sha256:parent.content_sha256,parent_raw_line:parent.raw_line,parent_raw_column:parent.raw_column,coordinate_basis:'raw-callframe-stack',coordinate_units:'UNKNOWN',coordinate_representation:'utf8-decoded-utf16-code-units-representation-only',source_frame:'UNKNOWN',caller_identity:'UNKNOWN',request_causality:'UNKNOWN',frames};
    }
    if(frames.length===1)return UNKNOWN_STACK(truncated?'TRUNCATED_UNMAPPED':'TOP_FRAME_ONLY','INSUFFICIENT',frames,truncated);
    if(frames.slice(1).every(f=>f.status==='MATCHED'&&f.candidate_role==='REPEATED_FRAME_SYNTAX_CANDIDATE'))return UNKNOWN_STACK(truncated?'TRUNCATED_UNMAPPED':'REPEATED_TOP_FRAME','INSUFFICIENT',frames,truncated);
    return UNKNOWN_STACK(truncated?'TRUNCATED_UNMAPPED':'PARENT_UNMAPPED','INSUFFICIENT',frames,truncated);
  };
  const finish=()=>{const result=guard(()=>{verify();for(const [sha,length] of retained){const bytes=read(path.join(destination(),sha+'.js'),caps.fileBytes,out).bytes;if(bytes.length!==length||hash(bytes)!==sha)bad('RETENTION');}persist(true);return true;});if(state==='UNKNOWN'&&ownedDir)guard(()=>persist(false));return result;};
  const finalize = result => state==='COMPLETE' ? result : UNKNOWN(reason);
  const finalizeStack = stackMatch => state==='COMPLETE' ? stackMatch : UNKNOWN_STACK(reason,'UNKNOWN',Array.isArray(stackMatch?.frames)?stackMatch.frames.map((f,i)=>UNKNOWN_FRAME(f?.frame_ordinal??i+1,reason)):[]);
  const summary = () => ({status:state,reason,scope:'regular-client-js-only',catalog_epoch:state==='COMPLETE'?initial.epoch:'UNKNOWN',catalog_manifest_sha256:state==='COMPLETE'?initial.catalogHash:'UNKNOWN',build_id_sha256:state==='COMPLETE'?initial.manifest.build_id_sha256:'UNKNOWN',file_count:state==='COMPLETE'?initial.entries.length:0,total_bytes:state==='COMPLETE'?initial.total:0,limits:caps,coordinate_units:'UNKNOWN'});
  return {match,matchStack,finish,finalize,finalizeStack,summary};
}
