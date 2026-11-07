// Purpose: bound one consuming-build client-JS catalog, exact public-location lookup and inert byte retention.
// Depends on: Node fs/path/crypto; Next's per-segment encodeURIComponent static URL contract.
// Used by: shop-real-gate's opt-in observer and registered c7-rsc-pageerror tests; never fetches or executes assets.
import fs from 'node:fs';
import path from 'node:path';
import { createHash } from 'node:crypto';

const CAPS = Object.freeze({ entries: 4096, files: 512, fileBytes: 8*1024*1024, totalBytes: 64*1024*1024, pathBytes: 1024, depth: 16, retainedBytes: 16*1024*1024, matches: 128 });
const hash = b => createHash('sha256').update(b).digest('hex');
const bad = code => { throw new Error(code); };
const UNKNOWN = reason => ({ status:'UNKNOWN', reason, asset_id:'UNKNOWN', content_sha256:'UNKNOWN', catalog_epoch:'UNKNOWN', catalog_manifest_sha256:'UNKNOWN', byte_length:'UNKNOWN', evidence_path:'UNKNOWN', coordinate_units:'UNKNOWN', source_frame:'UNKNOWN', browser_response_bytes:'UNKNOWN' });
const token = s => [s.dev,s.ino,s.size,s.mtimeNs,s.ctimeNs].join(':');

export function createClientAssetCatalog({ appRoot, evidenceRoot, origin, limits = {} }) {
  let state='COMPLETE', reason='NONE', initial, root, out, ownedDir, ownedToken, manifestHash=null, retainedBytes=0, calls=0, serial=0;
  const retained=new Map(), records=new Map(), caps={...CAPS};
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
    const object={status:state,reason,sealed, catalog_epoch:initial?.epoch??'UNKNOWN',catalog_manifest_sha256:initial?.catalogHash??'UNKNOWN',catalog:initial?.manifest??null,matches:[...records.values()].map(x=>state==='COMPLETE'?x:{error_id:x.error_id,...UNKNOWN(reason)})};
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
  const finish=()=>{const result=guard(()=>{verify();for(const [sha,length] of retained){const bytes=read(path.join(destination(),sha+'.js'),caps.fileBytes,out).bytes;if(bytes.length!==length||hash(bytes)!==sha)bad('RETENTION');}persist(true);return true;});if(state==='UNKNOWN'&&ownedDir)guard(()=>persist(false));return result;};
  const finalize = result => state==='COMPLETE' ? result : UNKNOWN(reason);
  const summary = () => ({status:state,reason,scope:'regular-client-js-only',catalog_epoch:state==='COMPLETE'?initial.epoch:'UNKNOWN',catalog_manifest_sha256:state==='COMPLETE'?initial.catalogHash:'UNKNOWN',build_id_sha256:state==='COMPLETE'?initial.manifest.build_id_sha256:'UNKNOWN',file_count:state==='COMPLETE'?initial.entries.length:0,total_bytes:state==='COMPLETE'?initial.total:0,limits:caps,coordinate_units:'UNKNOWN'});
  return {match,finish,finalize,summary};
}
